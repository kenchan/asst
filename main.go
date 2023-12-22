package main

import (
	"context"
	"log"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

/* openaiclientでcompletion apiを叩く */
func completion(client *azopenai.Client, prompt string) (string, error) {
	modelDeplymentName := "gpt-3.5-turbo-1106"
	resp, err := client.GetChatCompletions(
		context.TODO(),
		azopenai.ChatCompletionsOptions{
			Messages: []azopenai.ChatRequestMessageClassification{
				&azopenai.ChatRequestUserMessage{
					Content: azopenai.NewChatRequestUserMessageContent(prompt),
				},
			},
			DeploymentName: &modelDeplymentName,
		},
		nil,
	)
	if err != nil {
		log.Fatal(err)
		return "OpenAI API Error!!", err
	}
	return *resp.Choices[0].Message.Content, nil
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

	keyCredential := azcore.NewKeyCredential(openaiApiKey)
	openaiClient, err := azopenai.NewClientForOpenAI("https://api.openai.com/v1", keyCredential, nil)
	if err != nil {
		log.Fatal(err)
	}

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
