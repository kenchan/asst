package main

import (
	"context"
	"log"
	"os"

	openai "github.com/sashabaranov/go-openai"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

func completion(client *openai.Client, prompt string) (string, error) {
	resp, err := client.CreateChatCompletion(
		context.TODO(),
		openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo1106,
			Messages: []openai.ChatCompletionMessage{
				{
					Role: openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
		},
	)
	if err != nil {
		log.Fatal(err)
		return "OpenAI API Error!!", err
	}
	return resp.Choices[0].Message.Content, nil
}

func main() {
	appToken := os.Getenv("SLACK_APP_TOKEN")
	botToken := os.Getenv("SLACK_BOT_TOKEN")
	debugEnv := os.Getenv("DEBUG")
	openaiApiKey := os.Getenv("OPENAI_API_KEY")

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

				switch eventsAPIEvent.Type {
				case slackevents.CallbackEvent:
					innerEvent := eventsAPIEvent.InnerEvent
					switch ev := innerEvent.Data.(type) {
					case *slackevents.AppMentionEvent:
						msg, _ := completion(openaiClient, ev.Text)
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
				socketModeClient.Ack(*evt.Request)
			}
		}
	}()

	socketModeClient.Run()
}
