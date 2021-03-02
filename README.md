# logging
   


    
    package main

    import (
        "errors"
        "net/http"
        "github.com/peramic/logging"
        app "./go"
    )

    var log *logging.Logger

    func main() {
        log = logging.GetLogger("myApp")

        app.AddRoutes(logging.LogRoutes)

        log.Info("Server started")
        log.Error("Something got wrong")
        //log actual error true
        err:=errors.New("erors")
        log.WithError(err).Fatal("Something got wrong")
        router := app.NewRouter()

        log.Fatal(http.ListenAndServe(":8080", router))
    }
