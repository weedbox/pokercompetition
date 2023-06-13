package pokercompetition

import "fmt"

const (
	UnsetValue = -1
)

func LogJSON(msg string, jsonPrinter func() (string, error)) {
	json, _ := jsonPrinter()
	fmt.Printf("\n===== [%s] =====\n%s\n", msg, json)
}
