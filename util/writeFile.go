package util

import (
	"fmt"
	"os"
)

func WriteFile(fileName, msg string) {

	err := os.WriteFile(fileName, []byte(msg), os.ModePerm)

	if err != nil {
		fmt.Println("fileerr:", err)
	}
}
