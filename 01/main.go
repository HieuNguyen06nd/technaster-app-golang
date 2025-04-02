package main

import (
    "github.com/kataras/iris/v12"  // Import Iris
)

func main() {
    app := iris.New()
    app.Get("/", func(ctx iris.Context) {
        ctx.JSON(iris.Map{"message": "Hello Technaster!"})
    })
    app.Listen(":8080")
}