package main

type Response struct {
	Ok          bool     `json:"ok"`
	Result      []Result `json:"result"`
	Description string
}

type Result struct {
	Update_id	int		`json:"update_id"`
	Message		Message	`json:"message"`
	Inline		Inline 	`json:"inline_query"`
}

type Message struct {
	Message_id	int    `json:"message_id"`
	From		User   `json:"from"`
	Chat		Chat   `json:"chat"`
	Text		string `json:"text"`
}

type Inline struct {
	Id			string	`json:"id"`
	From		User	`json:"from"`
	Text		string	`json:"query"`
}

type User struct {
	Id       int
	Username string
}

type Chat struct {
	Id int
}
