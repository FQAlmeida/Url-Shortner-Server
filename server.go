package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"time"

	firebase "firebase.google.com/go"
	"firebase.google.com/go/auth"
	"google.golang.org/api/option"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

	"github.com/joho/godotenv"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Slug struct {
	ID     primitive.ObjectID `json:"id"`
	Slug   string             `json:"slug"`
	Domain string             `json:"redirect"`
	UserID string             `json:"uid"`
}
type SlugCreate struct {
	Slug   string `json:"slug"`
	Domain string `json:"redirect"`
	UserID string `json:"uid"`
}

type SlugPersistence struct {
	ID        primitive.ObjectID `json:"id"`
	Slug      string             `json:"slug"`
	Domain    string             `json:"redirect"`
	UserID    string             `json:"uid"`
	CreatedAt primitive.DateTime `json:"created_at"`
	UpdatedAt primitive.DateTime `json:"updated_at"`
}

type EndpointHit struct {
	Slug     Slug               `json:"slug"`
	HittedAt primitive.DateTime `json:"hitted_at"`
}

func _createSlug(slug *SlugPersistence, client *mongo.Client) error {
	var ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	collection := client.Database("url-shortner").Collection("slugs")
	_, err := collection.InsertOne(ctx, slug)
	return err
}

func createSlug(slug *Slug, client *mongo.Client) error {
	return _createSlug(&SlugPersistence{
		ID:        slug.ID,
		Slug:      slug.Slug,
		Domain:    slug.Domain,
		UserID:    slug.UserID,
		CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt: primitive.NewDateTimeFromTime(time.Now()),
	}, client)
}

func countSlugsWithin30Days(userID string, client *mongo.Client) (int64, error) {
	var ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	collection := client.Database("url-shortner").Collection("slugs")
	filter := bson.D{
		{
			Key:   "userid",
			Value: userID,
		},
		{
			Key: "createdat",
			Value: bson.D{
				{
					Key:   "$gte",
					Value: primitive.NewDateTimeFromTime(time.Now().AddDate(0, 0, -30)),
				},
			},
		},
	}
	count, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func deleteSlug(slug_id primitive.ObjectID, uid string, client *mongo.Client) error {
	var ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	collection := client.Database("url-shortner").Collection("slugs")
	filter := bson.D{{
		Key:   "userid",
		Value: uid,
	}, {
		Key:   "id",
		Value: slug_id,
	}}
	log.Println(filter)
	_, err := collection.DeleteOne(ctx, filter)
	return err
}
func _updateSlug(slug *Slug, client *mongo.Client) error {
	var ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	collection := client.Database("url-shortner").Collection("slugs")
	filter := bson.D{{
		Key:   "userid",
		Value: slug.UserID,
	}, {
		Key:   "id",
		Value: slug.ID,
	}}
	updater := bson.D{{Key: "$set", Value: bson.D{{
		Key:   "slug",
		Value: slug.Slug,
	}, {
		Key:   "domain",
		Value: slug.Domain,
	}, {
		Key:   "updatedat",
		Value: primitive.NewDateTimeFromTime(time.Now()),
	}}}}
	_, err := collection.UpdateMany(ctx, filter, updater)
	return err
}
func updateSlug(slug *Slug, client *mongo.Client) error {
	return _updateSlug(slug, client)
}

func createHit(slug *Slug, client *mongo.Client) error {
	var ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	collection := client.Database("url-shortner").Collection("hits")
	_, err := collection.InsertOne(ctx, &EndpointHit{
		Slug:     *slug,
		HittedAt: primitive.NewDateTimeFromTime(time.Now()),
	})
	return err
}

func _getSlugs(uid string, client *mongo.Client) ([]*SlugPersistence, error) {
	filter := bson.D{{
		Key:   "userid",
		Value: uid,
	}}
	return filterSlugs(filter, client)
}
func getSlugs(uid string, client *mongo.Client) ([]*Slug, error) {
	slugs_persist, err := _getSlugs(uid, client)
	if err != nil {
		return nil, err
	}
	slugs := make([]*Slug, 0, len(slugs_persist))
	for _, slug_persist := range slugs_persist {
		slugs = append(slugs, &Slug{
			ID:     slug_persist.ID,
			Slug:   slug_persist.Slug,
			Domain: slug_persist.Domain,
			UserID: slug_persist.UserID,
		})
	}
	return slugs, err
}
func _getSlug(slug string, client *mongo.Client) (*SlugPersistence, error) {
	filter := bson.D{{
		Key:   "slug",
		Value: slug,
	}}
	slugs, err := filterSlugs(filter, client)
	if len(slugs) == 0 {
		return nil, errors.New("Slug not found")
	}
	slug_persist := slugs[len(slugs)-1]
	return slug_persist, err
}
func getSlug(slug string, client *mongo.Client) (*Slug, error) {
	slug_persist, err := _getSlug(slug, client)
	if err != nil {
		return nil, err
	}
	slug_data := &Slug{
		ID:     slug_persist.ID,
		Slug:   slug_persist.Slug,
		Domain: slug_persist.Domain,
		UserID: slug_persist.UserID,
	}
	return slug_data, err
}

func filterSlugs(filter interface{}, client *mongo.Client) ([]*SlugPersistence, error) {
	var slugs []*SlugPersistence

	var ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	collection := client.Database("url-shortner").Collection("slugs")

	cur, err := collection.Find(ctx, filter)
	if err != nil {
		return slugs, err
	}
	for cur.Next(ctx) {
		var t SlugPersistence
		err := cur.Decode(&t)
		if err != nil {
			return slugs, err
		}

		slugs = append(slugs, &t)
	}

	if err := cur.Err(); err != nil {
		return slugs, err
	}

	// once exhausted, close the cursor
	cur.Close(ctx)

	if len(slugs) == 0 {
		slugs := []*SlugPersistence{}
		return slugs, nil
	}
	return slugs, nil
}

func loadMongoClient() (*mongo.Client, error) {
	db_username := url.QueryEscape(os.Getenv("DATABASE_USERNAME"))
	db_password := url.QueryEscape(os.Getenv("DATABASE_PASSWORD"))
	db_host := url.QueryEscape(os.Getenv("DATABASE_HOST"))

	db_uri := fmt.Sprintf(
		"mongodb+srv://%s:%s@%s/?retryWrites=true&w=majority", db_username, db_password, db_host,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx,
		options.Client().ApplyURI(
			db_uri))
	if err != nil {
		log.Fatal(err)
		return client, err
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}
	return client, err
}

func loadFirebaseClient() (*auth.Client, error) {
	// firebase_api_key := os.Getenv("FIREBASE_API_KEY")
	// firebase_app_id := os.Getenv("FIREBASE_APP_ID")
	// firebase_auth_domain := os.Getenv("FIREBASE_AUTH_DOMAIN")

	opts := option.WithCredentialsFile("url-shortner-fqa-firebase-adminsdk-zmv68-a3dcfdbe88.json")

	app, err := firebase.NewApp(context.Background(), nil, opts)
	if err != nil {
		log.Fatalf("error initializing app: %v\n", err)
	}
	fireAuth, err := app.Auth(context.Background())
	if err != nil {
		log.Fatalf("error initializing firebase auth: %v\n", err)
	}
	return fireAuth, err
}

func checkUserExists(userid string, firebaseAuth *auth.Client) (bool, error) {
	result, err := firebaseAuth.GetUser(context.Background(), userid)
	if auth.IsUserNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return result != nil, err
}

func main() {
	godotenv.Load(".env")
	client, err := loadMongoClient()
	if err != nil {
		log.Fatal(err)
	}
	firebaseAuth, err := loadFirebaseClient()
	if err != nil {
		log.Fatal(err)
	}

	r := gin.Default()
	r.Use(cors.Default())
	r.GET("/slugs", func(ctx *gin.Context) {
		id := ctx.Query("userid")
		exists, err := checkUserExists(id, firebaseAuth)
		if err != nil {
			ctx.JSON(500, gin.H{"err": err})
			return
		}
		if !exists {
			ctx.JSON(400, gin.H{"err": errors.New("USER not found")})
			return
		}
		slugs, err := getSlugs(id, client)
		if err != nil {
			ctx.JSON(500, gin.H{"err": err})
			return
		}
		ctx.JSON(200, slugs)
	})
	r.GET("/slug", func(ctx *gin.Context) {
		slug := ctx.Query("slug")
		slug_data, err := getSlug(slug, client)
		if err != nil {
			ctx.JSON(400, gin.H{})
			return
		}
		err = createHit(slug_data, client)
		if err != nil {
			ctx.JSON(500, gin.H{"err": err})
			return
		}
		ctx.JSON(200, slug_data)
	})
	r.POST("/slugs", func(ctx *gin.Context) {
		body := SlugCreate{}
		err := ctx.ShouldBindBodyWith(&body, binding.JSON)
		if err != nil {
			ctx.JSON(400, err)
			return
		}

		exists, err := checkUserExists(body.UserID, firebaseAuth)
		if err != nil {
			ctx.JSON(500, gin.H{"err": err})
			return
		}
		if !exists {
			ctx.JSON(400, gin.H{"err": errors.New("USER not found")})
			return
		}
		count, err := countSlugsWithin30Days(body.UserID, client)
		if err != nil {
			ctx.JSON(500, gin.H{"err": err})
			return
		}
		log.Println(count)
		if count >= 30 {
			ctx.JSON(400, gin.H{"err": errors.New("limit of slugs reached")})
			return
		}

		slug := &Slug{
			ID:     primitive.NewObjectID(), // use client to gen uuid
			UserID: body.UserID,
			Slug:   body.Slug,
			Domain: body.Domain,
		}

		err = createSlug(slug, client)
		if err != nil {
			ctx.JSON(500, gin.H{"err": err})
			return
		}

		ctx.JSON(200, slug)
	})
	r.DELETE("/slugs", func(ctx *gin.Context) {
		uid := ctx.Query("userid")

		exists, err := checkUserExists(uid, firebaseAuth)
		if err != nil {
			ctx.JSON(500, gin.H{"err": err})
			return
		}
		if !exists {
			ctx.JSON(400, gin.H{"err": errors.New("USER not found")})
			return
		}

		id_str := ctx.Query("id")
		id, err := primitive.ObjectIDFromHex(id_str)
		if err != nil {
			ctx.JSON(500, gin.H{"err": err})
			return
		}
		err = deleteSlug(id, uid, client)
		if err != nil {
			ctx.JSON(500, gin.H{"err": err})
			return
		}

		ctx.JSON(200, bson.D{})
	})
	r.PUT("/slugs", func(ctx *gin.Context) {
		slug := &Slug{}
		err := ctx.ShouldBindBodyWith(slug, binding.JSON)
		if err != nil {
			ctx.JSON(400, gin.H{"err": err})
			return
		}

		exists, err := checkUserExists(slug.UserID, firebaseAuth)
		if err != nil {
			ctx.JSON(500, gin.H{"err": err})
			return
		}
		if !exists {
			ctx.JSON(400, gin.H{"err": errors.New("USER not found")})
			return
		}

		err = updateSlug(slug, client)
		if err != nil {
			ctx.JSON(500, gin.H{"err": err})
			return
		}
		ctx.JSON(200, bson.D{})
	})

	r.Run() // listen and serve on 0.0.0.0:8080
	// log.Fatal(autotls.RunWithContext(ctx, r, "*"))
}
