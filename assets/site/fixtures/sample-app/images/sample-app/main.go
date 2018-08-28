/* this little web server is a sample application we use to test
gravity/k8s
it is basically "hello world" web app listening on http://0.0.0.0:5000
*/
package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
)

func main() {
	l, err := net.Listen("tcp", ":5000")
	if err != nil {
		fmt.Printf("cannot create listener: %v", err)
		os.Exit(1)
	}
	s := &http.Server{
		Handler: http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			fmt.Fprint(rw, "I am running under k8s!\n")
		}),
	}

	fmt.Println("Listening on http://localhost:5000 hit Ctrl+C to stop me...")

	if err = s.Serve(l); err != nil {
		fmt.Printf("cannot serve: %v", err)
		os.Exit(1)
	}

}
