package main

import (
	"context"
	"log"
	"os"
	"time"

	openai "github.com/sashabaranov/go-openai"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

func answer(client *openai.Client, prompt string, openaiAssistantID string) (string, error) {
	threadID, err := createThread(client, prompt)
	if err != nil {
		return "OpenAI API Error!!", err
	}

	runID, err := createAssistantRun(client, threadID, openaiAssistantID)
	if err != nil {
		return "OpenAI API Error!!", err
	}

	message, err := getMessage(client, threadID, runID)
	if err != nil {
		return "OpenAI API Error!!", err
	}

	return message, nil
}


func createThread(client *openai.Client, prompt string) (string, error) {
	resp, err := client.CreateThread(
		context.TODO(),
		openai.ThreadRequest{
			Messages: []openai.ThreadMessage{
				{
					Role: openai.ThreadMessageRoleUser,
					Content: prompt,
				},
			},
		},
	)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

func createAssistantRun(client *openai.Client, threadID string, assistantID string) (string, error) {
	resp, err := client.CreateRun(
		context.TODO(),
		threadID,
		openai.RunRequest{
			AssistantID: assistantID,
		},
	)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

func getMessage(client *openai.Client, threadID string, runID string) (string, error) {
	for {
		resp, err := client.RetrieveRun(
			context.TODO(),
			threadID,
			runID,
		)
		if err != nil {
			return "OpenAI API Error!!", err
		}
		if resp.Status == "completed" {
			break
		}
		time.Sleep(5 * time.Second)
	}

	resp, err := client.ListMessage(
		context.TODO(),
		threadID,
		nil,
		nil,
		nil,
		nil,
	)

	if err != nil {
		return "OpenAI API Error!!", err
	}

	return resp.Messages[0].Content[0].Text.Value, nil
}

func main() {
	appToken := os.Getenv("SLACK_APP_TOKEN")
	botToken := os.Getenv("SLACK_BOT_TOKEN")
	debugEnv := os.Getenv("DEBUG")
	openaiApiKey := os.Getenv("OPENAI_API_KEY")
	openaiAssistantID := os.Getenv("OPENAI_ASSISTANT_ID")

	isDebug := false
	if debugEnv != "" {
		isDebug = true
	}

	client := slack.New(
		botToken,
		slack.OptionAppLevelToken(appToken),
		slack.OptionDebug(isDebug),
		slack.OptionLog(log.New(os.Stdout, "api: ", log.Lshortfile|log.LstdFlags)),
	)

	socketModeClient := socketmode.New(
		client,
		socketmode.OptionDebug(isDebug),
		socketmode.OptionLog(log.New(os.Stdout, "socketmode: ", log.Lshortfile|log.LstdFlags)),
	)

	openaiClient := openai.NewClient(openaiApiKey)

	go func() {
		for evt := range socketModeClient.Events {
			switch evt.Type {
			case socketmode.EventTypeEventsAPI:
				eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
				if !ok {
					log.Printf("Ignored %+v\n", evt)
					continue
				}
				socketModeClient.Ack(*evt.Request)

				switch eventsAPIEvent.Type {
				case slackevents.CallbackEvent:
					innerEvent := eventsAPIEvent.InnerEvent
					switch ev := innerEvent.Data.(type) {
					case *slackevents.AppMentionEvent:
						msg, _ := answer(openaiClient, ev.Text, openaiAssistantID)
						_, _, err := client.PostMessage(
							ev.Channel,
							slack.MsgOptionText(msg, false),
							slack.MsgOptionTS(ev.TimeStamp),
						)
						if err != nil {
							log.Printf("failed posting message: %v", err)
							continue
						}
					}
				}
			}
		}
	}()

	socketModeClient.Run()
}
