# Migrations

Migrations should be ran one by one in alphanumeric order before the application has fully started up.

There are two options:
- Run migrations as part of the application startup process, by setting the `DB_MIGRATE` environment variable to `true`.
- Use the `dbmigrate` command to explicitly run the migrations at a time of your choosing.

## Why no up & down?

Some migration systems support "up" and "down" migrations. The idea is that "down" migrations
allow you to roll back the database to an earlier version. However, not all database operations
can actually be undone, any data deleted can't be restored. Making the entire model a bit awkward IMO.

I prefer modelling my migrations as an "append only" list of changes that were executed against the
database, if I need to roll back something I will add a new migration that undoes the previous one.

## The schema

Accompanying this readme you will find a `schema.gen.sql` file. This contains a dump of the current database schema,
and is kept up to date automatically. It allows us to get an overview of the current schema without having to comb
through individual migrations.

This file is purely meant for documentation purposes and is not used by the application in any way.
