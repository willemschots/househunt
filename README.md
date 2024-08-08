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

It's recommended to run the project locally using docker compose.

You will also need to have `npm` installed to build the frontend assets.

1. Create a directory to store data in the project root directory:
```sh
mkdir .localdev
```
2. Create a local `.env` file by copying `.env.sample`:
```sh
cp .env .env.sample
```
3. Follow the instructions in `.env` to configure the environment.
4. Build the frontend for the first time by running `make frontend`.
5. Run `docker compose up`. This will build the app and run it. You should see the database migrations being triggered and the HTTP server starting up.
6. Navigate to `http://localhost:8888` to see househunt in action.

## Frontend development

If you run househunt as described above, the container image will need to be rebuild each time the CSS and/or Javascript files are changed. Not ideal.

When working on the frontend it's convenient to run an additional server that will inject these files as necessary. This can be done as follows:

1. Run `npm run --prefix assets dev` to run the server.
2. Navigate to `http://localhost:3000` to find reach it.

This server will also:
- Hot reload Javascript and CSS.
- Auto refresh when HTML template files are changed.

The provided Docker Compose configuration will load HTML templates from a shared directory, any changes made to them will be visible in replies from the backend. In production you will want to cache the HTML templates in memory.
