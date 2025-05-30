package main

import (
	"bytes"
	"context"
	"flag"
	"html/template"
	"log"
	"notification-service/gen/go/campaign/v1"
	"notification-service/helper"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/mailgun/mailgun-go/v5"

	"github.com/rayhanadri/crowdfunding/user-service/pb"
	"github.com/streadway/amqp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/proto"
)

var (
	conn *amqp.Connection
	ch   *amqp.Channel
)

type CampaignTemplateData struct {
	Id              string
	UserId 			int32
	Title           string
	Description     string
	TargetAmount    int32
	MinDonation     int32
	CollectedAmount int32
	Deadline        time.Time
	Status          string
	Category        string
}

type UserTemplateData struct{
	Id int32
	Name string
	Email string
	Password string
}

// refactor error handling
func handleError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

var (
	serverAddr = flag.String("addr", "user-service-273575294549.asia-southeast2.run.app:443", "server address with port")
)

func main() {

	var err error
	// Find .env file
	err = godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Error loading .env file: %s", err)
	}

	// Get the rabbitmq URL from env
	rabbitMQURL := os.Getenv("RABBITMQ_URL")
	// Setting up a connection to RabbitMQ
	conn, err = amqp.Dial(rabbitMQURL)
	handleError(err, "Can't connect to AMQP")

	// If connection succeed, establish a channel, channel serves as the communication protocol over the connection
	ch, err = conn.Channel()
	handleError(err, "Can't create amqpChannel")

	// Declare a new exchange that later will push messages to queues
	// For the exchange type, we use "direct" type to send messages to queues by the exact matching on the routing key
	err = ch.ExchangeDeclare(
		"campaign-events", //name
		"direct",          // type
		true,              // durable
		false,             // auto-deleted
		false,             // internal
		false,             // no-wait
		nil,               // arguments
	)
	handleError(err, "Failed to declare an exhange")

	// Declare the queue for create campaign
	q, err := ch.QueueDeclare(
		"campaign_created_notification", // name
		true,                           // durable
		false,                           // delete when unused
		false,                           // exclusive
		false,                           // no-wait
		nil,                             // arguments
	)
	handleError(err, "Failed to declare a queue")

	// Bind the queue for create campaign
	err = ch.QueueBind(
		q.Name,             // queue name
		"campaign.created", // routing key
		"campaign-events",  // exchange name
		false,
		nil,
	)
	handleError(err, "Failed to bind a queue")

	// Declare the queue for delete campaign
	q2, err := ch.QueueDeclare(
		"campaign_deleted_notification", // name
		true,                           // durable
		false,                           // delete when unused
		false,                           // exclusive
		false,                           // no-wait
		nil,                             // arguments
	)
	handleError(err, "Failed to declare a queue")

	// Bind the queue for delete campaign
	err = ch.QueueBind(
		q2.Name,            // queue name
		"campaign.deleted", // routing key
		"campaign-events",  // exchange name
		false,
		nil,
	)
	handleError(err, "Failed to bind a queue")

	// Consume the queue
	msgs, err := ch.Consume(
		q.Name,                   // queue
		"notification-consumers", // consumer
		false,                    // auto-ack
		false,                    // exclusive
		false,                    // no-local
		false,                    // no-wait
		nil,                      // args
	)
	handleError(err, "Failed to register a consumer")

	// Consume the queue
	msgs2, err := ch.Consume(
		q2.Name,                         // queue
		"notification-consumers-delete", // consumer
		false,                           // auto-ack
		false,                           // exclusive
		false,                           // no-local
		false,                           // no-wait
		nil,                             // args
	)
	handleError(err, "Failed to register a consumer")

	var forever chan struct{}

	go func() {
		for d := range msgs {
			var response campaign.CreateCampaignResponse
			err := proto.Unmarshal(d.Body, &response)
			if err != nil {
				log.Printf("Failed to decode protobuf message: %v", err)
				continue
			}

			c := response.GetCreatedCampaign()
			if c == nil {
				log.Println("No CreatedCampaign found in response")
				continue
			}

			for _, c := range response.GetCreatedCampaign() {
				// Get User info from user service
				userInterface := ConnectUserService(c.UserId)
				log.Println(userInterface)
				SendHTMLTemplateEmail("create",userInterface.Email,c,"templates/campaign_create.html")
			}
			d.Ack(false)
		}
	}()

	go func() {
		for d := range msgs2 {
			var msg campaign.Notification
			err := proto.Unmarshal(d.Body, &msg)
			if err != nil {
				log.Printf("Failed to decode protobuf message delete: %v", err)
				continue
			}
			// Get User info from user service
			userInterface := ConnectUserService(msg.UserId)
			SendHTMLTemplateEmail("delete",userInterface.Email,&msg,"templates/campaign_delete.html")
			d.Ack(false)

		}
	}()

	log.Printf(" [*] Waiting for logs. To exit press CTRL+C")
	<-forever
}

func SendHTMLTemplateEmail(action string, email string, payload interface{},fileName string) {

	// Load and parse the HTML template
	tmpl, err := template.ParseFiles(fileName)
	if err != nil {
		log.Fatalf("❌ Failed to parse template: %v", err)
	}

	var data CampaignTemplateData
	if action == "create"{
		
		createdCampaign, ok := payload.(*campaign.Campaign)
		if !ok {
			log.Fatalf("failed to cast data campaign")
		}
		log.Println(createdCampaign)
		data = campaignToTemplateData(createdCampaign)
	} else if action == "delete"{
		
		id, ok := payload.(*campaign.Notification)
		if !ok {
			log.Fatalf("failed to cast data delete")
		}
		log.Println(id)
		data = campaignIDToTemplateData(id)
	}

	var htmlBody bytes.Buffer
	if err := tmpl.Execute(&htmlBody, data); err != nil {
		log.Fatalf("❌ Failed to render template: %v", err)
	}

	// Environment variables
	domain := os.Getenv("MAILGUN_DOMAIN")         // e.g., "crowdnotify.com"
	privateAPIKey := os.Getenv("MAILGUN_API_KEY") // Your private API key
	if domain == "" || privateAPIKey == "" {
		log.Fatal("❌ MAILGUN_DOMAIN or MAILGUN_API_KEY not set")
	}

	mg := mailgun.NewMailgun(privateAPIKey)

	sender := "CrowdfundingApp <crowdfunding@" + domain + ">"
	recipient := email
	var subject string
	switch action {
	case "create":
		subject = "🎉 Campaign Created"
	case "delete":
		subject = "🗑️ Campaign Deleted"
	default:
		subject = "Notification"
	}


	// Create message with HTML body
	message := mailgun.NewMessage(domain, sender, subject, "", recipient)
	message.SetHTML(htmlBody.String())

	// Send with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	log.Printf("Using Mailgun domain: '%s'\n", domain)

	log.Printf("%s", email)
	resp, err := mg.Send(ctx, message)
	if err != nil {
		log.Fatalf("❌ Failed to send email: %v", err)
	}

	log.Printf("✅ Email sent successfully: ID: %s\n", resp.ID)
}


func campaignToTemplateData(c *campaign.Campaign) CampaignTemplateData {
	return CampaignTemplateData{
		Id:              c.Id,
		Title:           c.Title,
		Description:     c.Description,
		TargetAmount:    c.TargetAmount,
		CollectedAmount: c.CollectedAmount,
		Deadline:        c.Deadline.AsTime(),
		Status:          helper.MapStatusDB(int32(c.Status)),
		Category:        helper.MapCategoryDB(int32(c.Category)),
		MinDonation:     c.MinDonation,
	}
}

func campaignIDToTemplateData(id *campaign.Notification) CampaignTemplateData {
    return CampaignTemplateData{
        Id: id.Id,
    }
}


func userToTemplateData(c *pb.UserResponse) UserTemplateData{
	return UserTemplateData{
		Id: c.Id,
		Name: c.Name,
		Email: c.Email,
		Password: c.Password,
	}
}

func ConnectUserService(id int32) UserTemplateData{
	// Start a connection to User-Service as Client
	flag.Parse()
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(nil, "")),
	}

	// Connect to non-interceptor server (MyService)
	conn, err := grpc.NewClient(*serverAddr, opts...)
	if err != nil {
		log.Fatalf("Failed to dial MyService server: %v", err)
	}
	defer conn.Close()
	client :=pb.NewUserServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	userResponse, err := client.GetUserByID(ctx,&pb.UserIdRequest{
		Id: id,
	})
	if err != nil{
		log.Fatalf("%v", err)
	}

	// transform user response to UserTemplateData
	result := userToTemplateData(userResponse)
	return result
}
