package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
)

type amqp struct{}

func (a amqp) PublishUserInserted(ctx context.Context, id int) {

	time.Sleep(time.Second * 2)
	log.Println("message publish")
}

type db struct{}

func (d db) InsertUser(ctx context.Context, name string) (int, error) {
	time.Sleep(time.Second * 5)
	log.Println("user insert")
	return 1, nil
}

type UserService struct {
	amqp amqp
	db   db

	doneWG sync.WaitGroup
}

func (s *UserService) RegisterUser(ctx context.Context, name string) error {
	log.Println("start user registration")

	userID, err := s.db.InsertUser(ctx, name)
	if err != nil {
		return fmt.Errorf("db insertion failed: %v", err)
	}

	s.PublishUserInserted(ctx, userID)
	return nil
}

func (s *UserService) PublishUserInserted(ctx context.Context, userId int) {
	s.doneWG.Add(1)
	go func() {
		defer s.doneWG.Done()
		defer func() {
			if err := recover(); err != nil {
				log.Printf("publishUserInserted recovered panic: %v\n", err)
			}
		}()
		s.amqp.PublishUserInserted(ctx, userId)
	}()
}

func (s *UserService) Stop(ctx context.Context) {
	log.Println("waiting for user service to finish")
	doneChan := make(chan struct{})
	go func() {
		s.doneWG.Wait()
		close(doneChan)
	}()

	select {
	case <-ctx.Done():
		log.Println("context done earlier then user service has stopped")
	case <-doneChan:
		log.Println("user service finished")

	}
}

func HandleUser(rw http.ResponseWriter, req *http.Request) {

}

func main() {
	userService := UserService{
		db:   db{},
		amqp: amqp{},
	}

	r := mux.NewRouter()
	r.HandleFunc("/tes", func(rw http.ResponseWriter, req *http.Request) {

		name := "some name"

		if err := userService.RegisterUser(req.Context(), name); err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}

		rw.WriteHeader(http.StatusOK)
		fmt.Fprintf(rw, "Success Testing!")
	}).Methods(http.MethodPost)

	srv := http.Server{}
	srv.Addr = ":8989"
	srv.Handler = r

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen and serve returned err: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("got interruption signal")
	if err := srv.Shutdown(context.TODO()); err != nil {
		log.Printf("server shutdown returned an err: %v\n", err)
	}

	userService.Stop(context.TODO())

	log.Println("final")

}
