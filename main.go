package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var lock sync.Mutex

func db() *mongo.Client {
	lock.Lock()
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")

	client, err := mongo.Connect(context.TODO(), clientOptions)

	if err != nil {
		log.Fatal(err)
	}

	err = client.Ping(context.TODO(), nil)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Connected to MongoDB!")
	lock.Unlock()
	return client
}

var userCollection = db().Database("Appointy").Collection("users")
var postCollection = db().Database("Appointy").Collection("posts")
var userCount = 0
var postCount = 0

type user struct {
	Id       string `json:id`
	Name     string `json:name`
	Email    string `json:email`
	Password string `json:password`
}

type post struct {
	Id      string `json:id`
	Caption string `json:caption`
	ImgUrl  string `json:imgurl`
	Time    string `json:time`
	Uid     string `json:uid`
}

func newUser(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		lock.Lock()
		w.Header().Set("Content-Type", "application/json")
		var new user
		userCount++
		err := json.NewDecoder(r.Body).Decode(&new)
		if err != nil {
			fmt.Print(err)
		}
		new.Id = strconv.Itoa(userCount)
		fmt.Println(new)
		ct := encrypt([]byte(new.Password), new.Name)
		new.Password = string(ct)
		insertResult, err := userCollection.InsertOne(context.TODO(), new)
		if err != nil {
			log.Fatal(err)
		}
		lock.Unlock()
		fmt.Println("User created successfully, ID: ", new.Id)
		json.NewEncoder(w).Encode(insertResult)
	}
}

func getUser(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		lock.Lock()
		w.Header().Set("Content-Type", "application/json")
		ids := r.URL.Path
		id := path.Base(ids)
		var result primitive.M
		err := userCollection.FindOne(context.TODO(), bson.D{{"id", id}}).Decode(&result)
		if err != nil {
			fmt.Println(err)
		}
		lock.Unlock()
		json.NewEncoder(w).Encode(result)

	}
}

func newPost(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		lock.Lock()
		w.Header().Set("Content-Type", "application/json")
		var new post
		postCount++
		err := json.NewDecoder(r.Body).Decode(&new)
		if err != nil {
			fmt.Print(err)
		}
		t := time.Now()
		new.Time = t.String()
		new.Id = strconv.Itoa(postCount)
		fmt.Println(new)
		insertResult, err := postCollection.InsertOne(context.TODO(), new)
		if err != nil {
			log.Fatal(err)
		}
		lock.Unlock()
		fmt.Println("Post inserted successfully, ID:", new.Id)
		json.NewEncoder(w).Encode(insertResult)
	}
}

func getPost(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		lock.Lock()
		w.Header().Set("Content-Type", "application/json")
		ids := r.URL.Path
		id := path.Base(ids)
		var result primitive.M
		err := postCollection.FindOne(context.TODO(), bson.D{{"id", id}}).Decode(&result)
		if err != nil {
			fmt.Println(err)
		}
		lock.Unlock()
		json.NewEncoder(w).Encode(result)
	}
}

func allPost(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		lock.Lock()
		w.Header().Set("Content-Type", "application/json")
		ids := r.URL.Path
		l := strings.Split(ids, "/")
		id := l[3]
		page, _ := strconv.Atoi(l[4])
		limit := 2
		i := 0
		cur, err := postCollection.Find(context.TODO(), bson.D{{"uid", id}})
		if err != nil {
			fmt.Println(err)
		}
		var results []*post
		for cur.Next(context.TODO()) {
			var elem post
			err := cur.Decode(&elem)
			if err != nil {
				log.Fatal(err)
			}
			if i == (page-1)*limit || i == (page-1)*limit+1 {
				results = append(results, &elem)
			}
			i++
		}
		if err := cur.Err(); err != nil {
			log.Fatal(err)
		}
		cur.Close(context.TODO())
		lock.Unlock()
		json.NewEncoder(w).Encode(results)
	}
}

func createHash(key string) string {
	hasher := md5.New()
	hasher.Write([]byte(key))
	return hex.EncodeToString(hasher.Sum(nil))
}

func encrypt(data []byte, passphrase string) []byte {
	block, _ := aes.NewCipher([]byte(createHash(passphrase)))
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err.Error())
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		panic(err.Error())
	}
	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return ciphertext
}

func decrypt(data []byte, passphrase string) []byte {
	key := []byte(createHash(passphrase))
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err.Error())
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err.Error())
	}
	nonceSize := gcm.NonceSize()
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		panic(err.Error())
	}
	return plaintext
}

func main() {
	http.HandleFunc("/users", newUser)
	http.HandleFunc("/users/", getUser)
	http.HandleFunc("/posts", newPost)
	http.HandleFunc("/posts/", getPost)
	http.HandleFunc("/posts/users/", allPost)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
