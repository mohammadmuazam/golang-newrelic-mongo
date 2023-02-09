package main

// create a go application which has basic crud operations in mongodb and track them using newrelic go agent
import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"github.com/newrelic/go-agent/v3/integrations/nrmongo"
	"github.com/newrelic/go-agent/v3/newrelic"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var db *mongo.Database
var Client *mongo.Client

type Book struct {
	Id     int     `json:"id"`
	Isbn   string  `json:"isbn"`
	Title  string  `json:"title"`
	Author *Author `json:"author"`
}

type Author struct {
	Firstname string `json:"firstname"`
	Lastname  string `json:"lastname"`
}

type GetBookResponse struct {
	Books []bson.M `json:"books"`
}

// func connectMongo() {
// 	nrMon := nrmongo.NewCommandMonitor(nil)

// 	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://localhost:27017/bookstore").SetMonitor(nrMon))
// 	if err != nil {
// 		panic(err)
// 	}
// 	if err := client.Ping(context.TODO(), readpref.Primary()); err != nil {
// 		panic(err)
// 	}

// 	fmt.Println("Connected to MongoDB!")
// 	Client = client
// }

func addBookToDB(app ...string) {
	if len(app) > 0 {
		fmt.Println(app[0])
	}
	booksCollection := Client.Database("bookstore").Collection("books")
	book := bson.M{"id": 1, "isbn": "448743", "title": "Book One", "author": bson.M{"firstname": "John", "lastname": "Doe"}}
	result, err := booksCollection.InsertOne(context.Background(), book)
	if err != nil {
		panic(err)
	}
	fmt.Println(result.InsertedID)
}

func main() {

	log.Print("Creating a newrelic application...")

	app, err := newrelic.NewApplication(
		newrelic.ConfigAppName("batman-mongo"),
		newrelic.ConfigLicense("0f3ed09a76c1e0100d821b5e37274a9aFFFFNRAL"),
		newrelic.ConfigDistributedTracerEnabled(true),
	)
	if err != nil {
		panic(err)
	}
	app.WaitForConnection(10 * time.Second)

	log.Print("Connected to newrelic...")

	nrMon := nrmongo.NewCommandMonitor(nil)
	ctx := context.Background()

	client, err := mongo.Connect(ctx, options.Client().SetTimeout(100*time.Millisecond).ApplyURI("mongodb://localhost:27017").SetMonitor(nrMon))
	if err != nil {
		fmt.Print(err)
	}
	defer client.Disconnect(ctx)

	// connectMongo()
	Client = client

	addBookToDB()

	r := chi.NewRouter()

	r.Use(newrelicMiddleware(app))

	r.Get("/books", getBooks)
	r.Post("/books", addBook)

	log.Println("Server started on the port 8001...")
	http.ListenAndServe(":8001", r)
}

func newrelicMiddleware(app *newrelic.Application) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			txn := app.StartTransaction(r.Method + " " + r.RequestURI)
			defer txn.End()
			ctx = newrelic.NewContext(ctx, txn)

			w = txn.SetWebResponse(w)
			txn.SetWebRequestHTTP(r)
			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		})
	}
}

func getBooks(w http.ResponseWriter, r *http.Request) {
	booksCollection := Client.Database("bookstore").Collection("books")
	ctx := r.Context()

	// ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	// defer cancel()
	// time.Sleep(10 * time.Second)
	// do a heavy db operation

	cur, err := booksCollection.Find(ctx, bson.M{})
	if err != nil {
		panic(err)
	}
	defer cur.Close(ctx)
	var books []bson.M
	for cur.Next(ctx) {
		var book bson.M
		err := cur.Decode(&book)
		if err != nil {
			panic(err)
		}
		books = append(books, book)
	}
	if err := cur.Err(); err != nil {
		panic(err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(GetBookResponse{Books: books})
}

func addBook(w http.ResponseWriter, r *http.Request) {
	booksCollection := Client.Database("bookstore").Collection("books")
	var book Book
	json.NewDecoder(r.Body).Decode(&book)
	_, err := booksCollection.InsertOne(r.Context(), book)
	if err != nil {
		panic(err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(book)
}
