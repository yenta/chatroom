package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID             uuid.UUID
	Addr           string
	EnterAt        time.Time
	MessageChannel chan Message
}
type Message struct {
	OwnerID uuid.UUID
	Content string
}

var (
	enteringChannel = make(chan *User)
	leavingChannel  = make(chan *User)
	messageChannel  = make(chan Message, 8)
)

func main() {
	listener, err := net.Listen("tcp", ":2020")
	if err != nil {
		panic(err)
	}
	go broadcaster()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		go handleConn(conn)
	}
}

func broadcaster() {
	users := make(map[*User]struct{})
	for {
		select {
		case user := <-enteringChannel:
			users[user] = struct{}{}
		case user := <-leavingChannel:
			delete(users, user)
			close(user.MessageChannel)
		case msg := <-messageChannel:
			for user := range users {
				if user.ID == msg.OwnerID {
					continue
				}
				user.MessageChannel <- msg
			}
		}
	}
}

func handleConn(conn net.Conn) {
	// new user
	user := &User{
		ID:             GenUserID(),
		Addr:           conn.RemoteAddr().String(),
		EnterAt:        time.Now(),
		MessageChannel: make(chan Message, 8),
	}
	// write
	go sendMessage(conn, user.MessageChannel)
	user.MessageChannel <- Message{OwnerID: user.ID, Content: fmt.Sprintf("Welcome, %+v\n", user)}
	messageChannel <- Message{OwnerID: user.ID, Content: "user:`" + user.ID.String() + "` has enter"}

	enteringChannel <- user

	var userActive chan struct{}
	go func() {
		d := 5 * time.Minute
		timer := time.NewTimer(d)
		for {
			select {
			case <-timer.C:
				conn.Close()
			case <-userActive:
				timer.Reset(d)
			}
		}
	}()

	input := bufio.NewScanner(conn)
	for input.Scan() {
		messageChannel <- Message{OwnerID: user.ID, Content: user.ID.String() + ":" + input.Text()}
		userActive <- struct{}{}
	}
	if err := input.Err(); err != nil {
		log.Println("Read Failed!:", err)
	}
	leavingChannel <- user
	messageChannel <- Message{OwnerID: user.ID, Content: "user:`" + user.ID.String() + "` has left"}
}

func sendMessage(conn net.Conn, ch <-chan Message) {
	for msg := range ch {
		fmt.Fprintln(conn, msg.Content)
	}
}

func GenUserID() uuid.UUID {
	return uuid.New()
}
