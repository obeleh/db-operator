# db-operator

This is a redesign of my first attampt to make a database operator: https://github.com/kabisa/postgresdb-operator

The goal of this operator is to provide databases for preview environments. It expects that there's already a `DbServer` available. This operator does not and (in the foreseable future) will not create Database Servers, RDS instances or absolve you of your DBA duties.

## Design

### Databases Diagram

![](./screenshots/databases-diagram.png)

### Backup Restore Diagram

![](./screenshots/databases-diagram.png)