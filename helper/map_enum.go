package helper

func MapCategoryDB(input int32) string{
	var result string
	// define enum type for database
	categories := map[int32]string{
		0: "unspecified",
		1: "education",
		2: "healthcare",
		3: "environment",
		4: "animals",
		5: "emergency",
		6: "community",
		7: "technology",
		8: "arts",
		9: "sports",
	}
	for index, category := range categories{
		if input == index{
			result = category
		}
	}
	return result 
}


func MapCateogryProto(input string) int32{
	var result int32
	categories := map[int32]string{
		0:"unspecified",
		1: "education",
		2: "healthcare",
		3: "environment",
		4: "animals",
		5: "emergency",
		6: "community",
		7: "technology",
		8: "arts",
		9: "sports",
	}
	for index, category := range categories{
		if input == category{
			result = index
		}
	}
	return result 
}

func MapStatusDB(input int32) string{
	var result string
	status := map[int32]string{
		0: "",
		1: "active",
		2: "paused",
		3: "completed",
		4: "cancelled",
	}
	for index, val := range status{
		if input == index{
			result = val
		}
	}
	return result 
}

func MapStatusProto(input string) int32{
	var result int32
	status := map[int32]string{
		0:"unspecified",
		1: "active",
		2: "paused",
		3: "completed",
		4: "cancelled",
	}
	for index, val := range status{
		if input == val{
			result = index
		}
	}
	return result 
}