# bashistdb

bashistdb stands for Bash History Database.

It is a very simple app that stores a bash_history file in a sqlite database.
It doesn't retain order, but instead keeps count of duplicate lines.

An example to get you going. First enable timestamping on bash history:

    $ export HISTTIMEFORMAT="%FT%T%z "
    $ echo 'HISTTIMEFORMAT="%FT%T%z "' >> ~/.bash_rc

Now load old entries¹²:

    $ history | go run bashistdb.go version.go

This will insert your bash_history into `database.sqlite` and show the 30 most frequent commands you used.

Then you probably want to add it to your PROMPT_COMMAND (PS1):

    $ export PROMPT_COMMAND="${PROMPT_COMMAND};"'echo ${USER} ${HOSTNAME%%.*} $(history 1 | sed "s/^[0-9]* *//")| ./bashistdb -s'

It is still incomplete.

I wrote this project to learn a bit about golang and SQL.


1: Old entries without timestamp will use a common timestamp, thus duplicates will not be inserted.
2: If an old entry spans across lines (very rare), you will have a segfault.
