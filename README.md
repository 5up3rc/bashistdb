# bashistdb

## Introduction

Bashistdb stands for Bash History Database.

Bashistdb stores bash history into a sqlite database.
It can either be run as standalone, or it can be run in a server-client mode,
where many clients can store their history into a single database.
In this mode, communication is compressed and encrypted, in order to be more
efficient and secure.

Bashistdb stores for each history line the time it was run, the user that run it
and the hostname. Currently it isn't meant to be secure against users. This means
that any user may be able to see commands that other users run, or store commands
under different user and hostnames. This is by design.

It is work in progress. Many important features are missing but it has a strong
foundation upon which new features can be build.

## Running

### Pre-requisites

Install sqlite3 on your machine and go get bashistdb:

    $ go get projects.30ohm.com/mrsaccess/bashistdb

If you are on a hardened machine, you may need instead:

    $ go get -u -ldflags '-extldflags=-fno-PIC' projects.30ohm.com/mrsaccess/bashistdb

Bashistdb needs your history to be timestamped in order to work. It understands
the RFC3339 time format.

In order to set up your bash to log and report RFC3339 timestamps, run:

    $ export HISTTIMEFORMAT="%FT%T%z "
    $ echo 'HISTTIMEFORMAT="%FT%T%z "' >> ~/.bash_rc

While this is enough to get you started, if you want to import your old, non
timestamped history, you will have to create some distinct timestamps. A tool
is provided.

    $ go get projects.30ohm.com/mrsaccess/bashistdb/tools/addTimestamp2Hist

*Copy* your old history and then replace it:

    $ cp ~/.bash_history ~/.bash_history.bak
    $ addTimestamp2Hist -f ~/.bash_history.bak -since 24 > ~/.bash_history

This will create timestamps for your current commands that span equally accross
the 24 last months.

### Local mode:

Import your current history:

    $ history | bashistdb

Check some stats:

    $ bashistdb

Restore your history file:

    $ bashistdb --restore > ~/.bash_history

That's it. You can import your history as many times as you want. It is very fast
and only new lines will be added.

If you prefer to add your history as it happens, then try:

    $ export PROMPT_COMMAND="${PROMPT_COMMAND}; history 1 | bashistdb"

### Server - Client mode:

Start your server¹:

    $ bashistdb -s -p "passphrase"

From your client machine run bashistdb in client mode:

    $ history | bashistdb -c <SERVER> -p "passphrase"

Optionally you may set your passphrase as an environment variable. Get
some stats from server:

    $ export BASHISTDB_KEY=passphrase
    $ bashistdb -c <SERVER>

If you want to sent your history at the server as it happens, you better
start bashistdb client in the background in order to avoid delays:

    $ export PROMPT_COMMAND="${PROMPT_COMMAND}; (history 1 | bashistdb -c <SERVER> -p passphrase &)"

Messages are encrypted using NaCl secret-key authenticated encryption and
scrypt key derivation.
Check <https://github.com/andmarios/crypto/nacl/saltsecret> if you are
interested for a higher lever wrapper for golang's crypto/nacl/secretbox.

1: Currently bashistdb listens to all network interfaces (0.0.0.0). It
will get a listen address configuration option in the future.

### Knobs

Run `bashistdb -h` to get a glimpse of available options. They are easy to understand.

## License

Copyright (c) 2015, Marios Andreopoulos.

This file is part of bashistdb.

Bashistdb is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

Bashistdb is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with bashistdb.  If not, see <http://www.gnu.org/licenses/>.
