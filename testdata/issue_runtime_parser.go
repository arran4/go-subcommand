package parserpkg

func Parse(value string) (string, error) {
	return "imported:" + value, nil
}

func Gen() (string, error) {
	return "generated", nil
}
