package main

import (
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/gin"
	"github.com/gin-gonic/gin"
)

var initialized = false
var ginLambda *ginadapter.GinLambda

type dialogResponse struct {
	Text string `json:"fulfillmentText"`
	//	Messages []textUp 	`json:"fulfillmentMessages"`
}

type text struct {
	Text []string `json:"text`
}

func Handler(req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {

	if !initialized {
		// stdout and stderr are sent to AWS CloudWatch Logs
		log.Printf("Gin cold start")
		r := gin.Default()
		r.GET("/helloWorld", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"message": "pong",
			})
		})

		r.POST("/helloWorld", func(c *gin.Context) {
			dResp := dialogResponse{
				Text: "This is a Lambda response",
			}
			c.JSON(200, dResp)
		})

		ginLambda = ginadapter.New(r)
		initialized = true
	}

	// If no name is provided in the HTTP request body, throw an error
	return ginLambda.Proxy(req)
}

func main() {
	lambda.Start(Handler)
}
