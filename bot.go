package main

import (
	"encoding/json"
	"fmt"
	"github.com/garyburd/redigo/redis"
	"github.com/alexurquhart/rlimit"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func sendCommand(method, token string, params url.Values) ([]byte, error) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/%s?%s",
		token, method, params.Encode())

	timeout := 35 * time.Second

	client := http.Client{
		Timeout: timeout,
	}

	resp, err := client.Get(url)
	if err != nil {
		return []byte{}, err
	}
	resp.Close = true
	defer resp.Body.Close()
	json, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return []byte{}, err
	}

	return json, nil
}

func (bot *Bot) Commands(input string, author string) string {
	markov := Markov{20}
	word := strings.Split(input, " ")

	commandParts := strings.Split(word[0], "@")
	command := commandParts[0]

	if len(commandParts) > 1 {
		botName := commandParts[1]
		if (botName != name) { // Bail out if the command was directed at another bot
			return ""
		}
	}

	if command == "/chobotta" {
		var seed string
		if (len(word) == 1) {
			seed, _ = redis.String(bot.Connection.Do("RANDOMKEY"))
		} else {
			seed = strings.Join(word[1:], " ") // Removes the initial command
		}
		return markov.Generate(seed, bot.Connection)
	} else if word[0] == "/chorate" && len(word) >= 2 && author == admin {
		n, err := strconv.Atoi(word[1])
		if err != nil || n < 0 || n > 100 {
			return "Use a number between 0 and 100."
		} else {
			bot.Chance = n
			log.Printf("Bot rate: %v\n", bot.Chance)
			return "Rate set"
		}
	} else if command == "/chosource" {
		return "Source: https://github.com/CapacitorSet/ritalobot"
	} else {
		return ""
	}
}

type Bot struct {
	Token      string
	Connection redis.Conn
	Chance     int
}

func (bot Bot) GetUpdates() []Result {
	offset, _ := redis.String(bot.Connection.Do("GET", "update_id"))

	params := url.Values{}
	params.Set("offset", offset)
	params.Set("timeout", strconv.Itoa(30))

	resp, err := sendCommand("getUpdates", token, params)
	if err != nil {
		log.Println(err)
		return nil
	}

	var updatesReceived Response
	json.Unmarshal(resp, &updatesReceived)

	if !updatesReceived.Ok {
		log.Println("updatesReceived not OK")
		log.Println(updatesReceived.Description)
		return nil
	}

	updates := updatesReceived.Result

	if len(updates) != 0 {
		updateID := updates[len(updates)-1].Update_id + 1
		bot.Connection.Do("SET", "update_id", updateID)
	}

	return updates
}

func (bot Bot) Listen() {
	var err error

	rand.Seed(time.Now().UnixNano())
	bot.Chance = chance

	tmp := ":" + strconv.Itoa(port)
	bot.Connection, err = redis.Dial(connection, tmp)
	if err != nil {
		fmt.Println("connection to redis failed")
		log.Fatal(err)
	}
	fmt.Printf("redis connection: %v | port is %v\n", connection, port)
	fmt.Printf("chance rate %v%!\n", bot.Chance)

	bot.Poll()

}

func (bot Bot) Poll() {
	markov := Markov{10}

	// https://github.com/alexurquhart/rlimit/blob/master/examples/blocking/blocking.go
	// Create a new limiter that ticks every 200ms, limited to 25 times per minute
	interval := time.Duration(200) * time.Millisecond
	resetInterval := time.Duration(1) * time.Minute
	limiter := rlimit.NewRateLimiter(interval, 25, resetInterval)

	for {
		updates := bot.GetUpdates()

		if updates != nil {
			for _, update := range updates {
				text := fetchText(update)
				author := fetchAuthor(update)
				markov.StoreUpdate(text, bot.Connection)

				i := 0
				var response string
				log.Println(text)
				for i == 0 || (strings.Trim(response, " \t") == strings.Trim(text, " \t") && i < 5) {
					response = process(text, isInline(update), author, markov, bot)
					i++
				}

				if response != "" && i != 5 {
					log.Println("Done")
					if update.Message.Text != "" {
						limiter.Wait()
						bot.Say(response, update.Message.Chat.Id)
					} else {
						bot.SayInline(response, update.Inline.Id)
					}
				}
			}
		}
	}
}

func isInline(item Result) bool {
	return item.Message.Text == ""
}

func fetchText(item Result) string {
	if isInline(item) {
		return item.Inline.Text
	} else {
		return item.Message.Text
	}
}

func fetchAuthor(item Result) User {
	if isInline(item) {
		return item.Inline.From
	} else {
		return item.Message.From
	}
}

func process(text string, inline bool, author User, markov Markov, bot Bot) string {
	if strings.HasPrefix(text, "/cho") {
		return bot.Commands(text, author.Username)
	} else if inline || (rand.Intn(100) <= bot.Chance) {
		var seed string
		if (inline) {
			seed = text
		} else {
			seed, _ = redis.String(bot.Connection.Do("RANDOMKEY"))
		}

		return markov.Generate(seed, bot.Connection)
	} else {
		return ""
	}
}

func (bot Bot) Say(text string, chat int) bool {
	if strings.HasPrefix(text, "!kickme") || strings.HasPrefix(text, "/AttivaTelegramPremium") {
		return true
	}

	var responseReceived struct {
		Ok          bool
		Description string
	}

	params := url.Values{}

	params.Set("chat_id", strconv.Itoa(chat))
	params.Set("text", text)
	resp, err := sendCommand("sendMessage", token, params)

	err = json.Unmarshal(resp, &responseReceived)
	if err != nil {
		return false
	}

	if !responseReceived.Ok {
		return false
		// fmt.Errorf("chobot: %s\n", responseReceived.Description)
	}

	return responseReceived.Ok
}

func (bot Bot) SayInline(text string, id string) {
	text_params, _ := json.Marshal(map[string]string{
		"type": "article",
		"id": "a",
		"title": text,
		"message_text": text})

	params := url.Values{}
	params.Set("inline_query_id", id)
	params.Set("results", fmt.Sprintf("[%s]", string(text_params)))
	params.Set("cache_time", "0")

	sendCommand("answerInlineQuery", token, params)
}
