package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
	"unsafe"

	badger "github.com/dgraph-io/badger/v3"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

type Message struct {
	Date    time.Time
	Content string
	ID      uint64
	UserIP  net.IP
	//attatchments [][]byte
}

type Message_post struct {
	Content string
	Address string
}

type New_message struct {
	Data []byte
	ID   uint64
}

var db *badger.DB

var PORT string

var messages = make(chan New_message)
var deleted_messages = make(chan string)

func main_handler(ctx *fiber.Ctx) error {
	return ctx.SendString("Go away")
}

func userSet_handler(ctx *fiber.Ctx) error {

	address := ctx.Query("address")
	username := ctx.Query("username")

	if address == "" || username == "" {
		return fiber.NewError(fiber.StatusBadRequest, "You must provide a username and address in the params")
	}

	ipAddr := net.ParseIP(address)

	if ipAddr == nil {
		return fiber.NewError(fiber.StatusBadRequest, "Could not parse the provided address")
	}

	if !hasIP(ipAddr) {
		return fiber.NewError(fiber.StatusBadRequest, "The provided address does not exist on this network")
	}

	err := db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(ipAddr.String()), []byte(username))
	})

	if err != nil {
		return fiber.NewError(fiber.StatusFailedDependency, "Could not set new user")
	}

	return ctx.SendString("Username set to " + username)
}

func userGet_handler(ctx *fiber.Ctx) error {
	address := ctx.Params("address")

	var username []byte

	err := db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(address))

		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			username = val
			return nil
		})
	})

	if err != nil {
		return fiber.NewError(fiber.StatusFailedDependency, "Could not get username from database")
	}

	return ctx.Send(username)
}

func messageGet_handler(ctx *fiber.Ctx) error {

	id := ctx.Params("id")

	var msg []byte

	err := db.View(func(txn *badger.Txn) error {

		id, err := byteID(id)

		if err != nil {
			return err
		}

		item, err := txn.Get(id)

		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			msg = val
			return nil
		})
	})

	if err != nil && err.Error() == "Key not found" {
		return fiber.NewError(fiber.StatusBadRequest, "Could not get message. perhaps the id was invalid.")
	}

	ctx.Append("application/json")
	_, err = ctx.Write(msg)
	return err
}

func messagePost_handler(ctx *fiber.Ctx) error {

	ctx.Accepts("application/json")

	var msg Message_post
	err := json.Unmarshal(ctx.Request().Body(), &msg)

	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Could not parse request body")
	}

	if msg.Content == "" || msg.Address == "" {
		return fiber.NewError(fiber.StatusBadRequest, "body must include a valid content and address field")
	}

	sq, err := db.GetSequence([]byte("id"), 1000)

	defer sq.Release()

	if err != nil {
		return err
	}

	id, err := sq.Next()

	if err != nil {
		return err
	}

	ipAddr := net.ParseIP(msg.Address)

	if ipAddr == nil {
		return fiber.NewError(fiber.StatusFailedDependency, "Could not prase ip address")
	}

	m, err := json.Marshal(&Message{
		Content: msg.Content,
		ID:      id,
		Date:    time.Now(),
		UserIP:  ipAddr,
	})

	if err != nil {
		return fiber.NewError(fiber.StatusFailedDependency, "Could not parse json to write to db")
	}

	err = db.Update(func(txn *badger.Txn) error {
		buff := make([]byte, unsafe.Sizeof(id))
		binary.BigEndian.PutUint64(buff, uint64(id))

		return txn.Set(buff, m)
	})

	if err != nil {
		return fiber.NewError(fiber.StatusFailedDependency, "Could not write message to database")
	}

	new_msg := New_message{
		Data: m,
		ID:   id,
	}

	go func() {
		messages <- new_msg
	}()

	return ctx.SendString("Message posted")
}

func messageAll_handler(ctx *fiber.Ctx) error {
	var msgs []*Message

	err := db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions

		iter := txn.NewIterator(opts)

		defer iter.Close()

		for iter.Rewind(); iter.Valid(); iter.Next() {
			item := iter.Item()

			err := item.Value(func(val []byte) error {

				// for whatever reason the last element always is empty so this check prevents null json outputs
				if string(val) == "" {
					return nil
				}

				var msg *Message
				json.Unmarshal(val, &msg)

				if msg != nil {
					msgs = append(msgs, msg)
				}

				return nil
			})

			if err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return fiber.NewError(fiber.StatusFailedDependency, "Could not get messages")
	}

	ctx.Accepts("application/json")
	ctx.Append("Content-Type", "application/json")

	return ctx.JSON(msgs)
}

func messageDelete_handler(ctx *fiber.Ctx) error {

	id := ctx.Params("id")

	if id == "" {
		return fiber.NewError(fiber.StatusBadRequest, "You must provide a valid message id")
	}

	err := db.Update(func(txn *badger.Txn) error {
		id, err := byteID(id)

		if err != nil {
			return err
		}

		return txn.Delete(id)
	})

	if err != nil {
		return fiber.NewError(fiber.StatusFailedDependency, "Could not delete message from database")
	}

	go func() {
		deleted_messages <- id
	}()

	ctx.SendString("Message deleted")
	return nil
}

func event_handler(ctx *fiber.Ctx) error {

	ctx.Append("Access-Control-Allow-Origin", "*")
	ctx.Append("Content-Type", "text/event-stream")
	ctx.Append("Cache-Control", "no-cache")
	ctx.Append("Connection", "keep-alive")

	select {
	case m := <-messages:
		msg := fmt.Sprintf("event: message\nid: %d\ndata: %s\n\n", m.ID-1, string(m.Data))
		r := bytes.NewReader([]byte(msg))
		ctx.SendStream(r)
	case id := <-deleted_messages:
		msg := fmt.Sprintf("event: message_deleted\ndata: %s\n\n", id)
		r := bytes.NewReader([]byte(msg))
		ctx.SendStream(r)
	}

	return nil
}

func fatalErr(err error, msg string) {
	if err != nil {
		log.Fatal(msg + " Error: " + err.Error())
	}
}

func init() {

	flag.StringVar(&PORT, "port", ":80", "must be a valid port address")
	flag.Parse()

	log.SetPrefix("[Server] ")
	log.SetFlags(log.Ltime)
}

func main() {

	if !strings.HasPrefix(PORT, ":") {
		PORT = ":" + PORT
	}

	log.Println("Server has start on port " + PORT)

	var err error
	db, err = badger.Open(badger.DefaultOptions("database"))

	fatalErr(err, "Could not open database")

	defer db.Close()

	app := fiber.New()

	// allows all origins
	app.Use(cors.New())

	app.All("/", main_handler)
	app.Get("/message/all", messageAll_handler)
	app.Get("/message/get/:id", messageGet_handler)
	app.Post("/user/set/", userSet_handler)
	app.Get("/user/get/:address", userGet_handler)
	app.Get("/events/", event_handler)
	app.Post("/message/post", messagePost_handler)
	app.Delete("/message/delete/:id", messageDelete_handler)

	app.Static("/static/", "./web_client/public/")

	log.Fatal(app.Listen(":80"))
}
