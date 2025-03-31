package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/piotrbelina/code-for-blog/kubernetes-series/01-basic/client"
)

func main() {
	hc := http.Client{}
	c, err := client.NewClientWithResponses("http://127.0.0.1:8000", client.WithHTTPClient(&hc))
	if err != nil {
		log.Fatal(err)
	}

	resp, err := c.PostTodoWithResponse(context.TODO(), client.PostTodoJSONRequestBody{Title: "Test"})
	if err != nil {
		log.Fatal(err)
	}
	if resp.StatusCode() != http.StatusCreated {
		log.Fatalf("Expected HTTP 201 but received %d", resp.StatusCode())
	}
	fmt.Printf("POST /todos resp.JSON201: %v\n", resp.JSON201)

	resp2, err := c.GetTodosWithResponse(context.TODO())
	if err != nil {
		log.Fatal(err)
	}
	if resp2.StatusCode() != http.StatusOK {
		log.Fatalf("Expected HTTP 200 but received %d", resp2.StatusCode())
	}
	fmt.Printf("GET  /todos resp.JSON200: %v\n", resp2.JSON200)
}
