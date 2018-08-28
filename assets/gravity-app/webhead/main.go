package main

import (
	"fmt"
	"log"
	"net/http"
)

const pageTemplate = `
<html>
<title>Web Head</title>
<body>
%v
</body>
</html>
`

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, pageTemplate, "Hello! I am <b>the</b> webhead.")
	})
	fmt.Printf("I am listening on http://0.0.0.0:8080\n")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
