# bashistdb

bashistdb stands for Bash History Database.

It is a very simple app that stores a bash_history file in a sqlite database.
It doesn't retain order, but instead keeps count of duplicate lines.

An example to get you going:

    $ cat ~/.bash_history| go run bashistdb.go

This will insert your bash_history into `database.sqlite` and show the 30 most frequent commands you used.


I wrote this project to learn a bit about golang and SQL.
