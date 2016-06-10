package main

import (
	"encoding/json"
	"fmt"
	"github.com/garyburd/redigo/redis"
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

func (bot *Bot) Commands(input string, chat int) {
	markov := Markov{20}
	word := strings.Split(input, " ")

	seed := strings.Join(word[1:], " ") // Removes the initial command

	commandParts := strings.Split(word[0], "@")
	var command = commandParts[0]

	if len(commandParts) > 1 {
		var botName = commandParts[1]
		if (botName != name) { // Bail out if the command was directed at another bot
			return
		}
	}

	if command == "/chobot" && len(word) >= 2 {
		text := markov.Generate(seed, bot.Connection)
		bot.Say(text, chat)
	} else if word[0] == "/chorate" && len(word) >= 2 {
		n, err := strconv.Atoi(word[1])
		if err != nil || n <= 0 || n > 100 {
			bot.Say("Use a number between 1 and 100.", chat)
		} else {
			bot.Chance = n
			log.Printf("Bot rate: %v\n", bot.Chance)
			bot.Say("Rate set", chat)
		}
	} else if command == "/chosource" {
		text := fmt.Sprintf("Author: %v \nSource: %v",
			"@blackdev1l",
			"https://github.com/blackdev1l/ritalobot")
		bot.Say(text, chat)
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
	}

	var updatesRecieved Response
	json.Unmarshal(resp, &updatesRecieved)

	if !updatesRecieved.Ok {
		err = fmt.Errorf("chobot: %s\n", updatesRecieved.Description)
		return nil
	}

	var updates = updatesRecieved.Result
	if len(updates) != 0 {

		updateID := updates[len(updates)-1].Update_id + 1
		bot.Connection.Do("SET", "update_id", updateID)

		return updates

	}
	return nil
}

func (bot Bot) Say(text string, chat int) (bool, error) {

	var responseRecieved struct {
		Ok          bool
		Description string
	}

	params := url.Values{}

	params.Set("chat_id", strconv.Itoa(chat))
	params.Set("text", text)
	resp, err := sendCommand("sendMessage", token, params)

	err = json.Unmarshal(resp, &responseRecieved)
	if err != nil {
		return false, err
	}

	if !responseRecieved.Ok {
		return false, fmt.Errorf("chobot: %s\n", responseRecieved.Description)
	}

	return responseRecieved.Ok, nil
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
	for {
		updates := bot.GetUpdates()
		if updates != nil {
			markov.StoreUpdates(updates, bot.Connection)
			if strings.HasPrefix(updates[0].Message.Text, "/cho") {
				bot.Commands(updates[0].Message.Text,
					updates[0].Message.Chat.Id)

			} else if rand.Intn(100) <= bot.Chance {
				seed, _ := redis.String(bot.Connection.Do("RANDOMKEY"))

				chat := updates[len(updates)-1].Message.Chat.Id
				out_text := markov.Generate(seed, bot.Connection)
				bot.Say(out_text, chat)
			}

		}
	}
}
