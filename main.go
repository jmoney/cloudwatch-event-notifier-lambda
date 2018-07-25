// Copyright 2018 Jonathan Monette
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//        http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/jmoney8080/go-gadget-slack"
)

var (
	// Info Logger
	Info *log.Logger
	// Warning Logger
	Warning *log.Logger
	// Error Logger
	Error *log.Logger

	slackClient               *slack.Client
	slackAttachmentsChunkSize int
	slackMonitorChannel       string
)

// EMREventDetail struct to extract out a few key attributes
type EMREventDetail struct {
	Severity string `json:"severity"`
	State    string `json:"state"`
	Message  string `json:"message"`
}

func init() {

	Info = log.New(os.Stdout,
		"[INFO]: ",
		log.Ldate|log.Ltime|log.Lshortfile)

	Warning = log.New(os.Stdout,
		"[WARNING]: ",
		log.Ldate|log.Ltime|log.Lshortfile)

	Error = log.New(os.Stderr,
		"[ERROR]: ",
		log.Ldate|log.Ltime|log.Lshortfile)

	slackClient = slack.New(http.Client{Timeout: 10 * time.Second}, os.Getenv("SLACK_WEBHOOK"))
	slackAttachmentsChunkSize = 100
	slackMonitorChannel = os.Getenv("SLACK_MONITOR_CHANNEL")
}

func main() {
	lambda.Start(HandleRequest)
}

// HandleRequest function that the lambda runtime service calls
func HandleRequest(ctx context.Context, event events.CloudWatchEvent) error {
	slackAttachments := []slack.Attachment{}

	if event.Source == "aws.emr" {
		emrEventDetail := EMREventDetail{}
		err := json.Unmarshal(event.Detail, &emrEventDetail)
		if err != nil {
			Error.Println(err)
			return nil
		}

		color := "good"
		if emrEventDetail.Severity == "ERROR" {
			color = "danger"
		}

		slackAttachment := slack.Attachment{
			Color:      color,
			Title:      event.DetailType,
			Text:       emrEventDetail.Message,
			Footer:     os.Getenv("AWS_LAMBDA_FUNCTION_NAME"),
			FooterIcon: "https://d1d05r7k0qlw4w.cloudfront.net/dist-cbe91c5a8477701757ff6752aae4c6f892018972/img/favicon.ico",
			Ts:         time.Now().UnixNano() / int64(time.Second),
			AttachmentField: []slack.AttachmentField{
				{
					Title: "AccountID",
					Value: event.AccountID,
					Short: true,
				},
				{
					Title: "Region",
					Value: event.Region,
					Short: true,
				},
				{
					Title: "State",
					Value: emrEventDetail.State,
					Short: true,
				},
				{
					Title: "Time",
					Value: event.Time.String(),
					Short: true,
				},
			},
		}
		slackAttachments = append(slackAttachments, slackAttachment)
	}

	if len(slackAttachments) != 0 {
		// Here we are chunking up the attachments.  Slack only allows 100 attachments in one post. While that'd be insane and absurd to do, it's a known limit
		// we can easily account for in the code
		for i := 0; i < len(slackAttachments); i += slackAttachmentsChunkSize {
			end := i + slackAttachmentsChunkSize
			if end > len(slackAttachments) {
				end = len(slackAttachments)
			}

			chunkedSlackAttachments := slackAttachments[i:end]
			payload := slack.Payload{
				Channel:     slackMonitorChannel,
				Attachments: chunkedSlackAttachments,
			}
			resp, err := (*slackClient).Send(payload)
			if err != nil {
				Error.Println(err)
			} else {
				Info.Println(resp)
			}
		}
	} else {
		Warning.Println("No Slack Sent")
	}
	return nil
}
