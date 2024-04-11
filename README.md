# Househunt - A complete Go web application

After learning Go's syntax and building a few single package projects, you now want to build a web application.

However, it's daunting to start and organize your first bigger Go project:

- Where do you put your files?
- How do structure your application?
- How do you combine the different packages?
- How do you write unit and integration tests?
- .. and many more questions.

One way to learn all this this is by reading existing source code.

There are a lot of existing Go repositories out there, but most of them are not made for self-learning. They are either overwhelmingly big, too trivial or lack documentation.

This is where the `househunt` project comes in. Househunt is a fully featured Go web application with the explicit aim of being __an example web application__.

It uses the standard library where possible and is well documented.

## The househunt project

Househunt will be a web app where real estate agents can post listings and house hunter can respond to them. You can find out more in [this article](https://www.willem.dev/articles/example-web-application-project/).

## The production server

You can see househunt in action at [examplego.com](https://examplego.com), this is considered the production environment for the app.

## Build in public

Househunt is being build in public by [Willem Schots](https://www.willem.dev/), you can follow along [on Twitter](https://www.x.com/willemschots). 

## Running househunt locally

The recommended way to run the project locally is using docker compose, this will ease management of env variable based settings.

1. Create a directory for storing the local data in the project root directory:
```sh
mkdir .localdev
```
2. Configure `.env` file based on the data in `.env.sample`.
3. Run `docker compose up`. This will build the app and run it. You should see the database migrations being triggered and the HTTP server starting up.
4. Navigate to `http://localhost:8888` to see househunt in action.

