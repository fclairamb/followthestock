# Follow the stock

It follows some stocks on [boursorama](http://www.boursorama.com) and send some alerts through XMPP when it detects some changes over the subscribed percentage threshold.

# Command line interface

Here are the command line interfaces arguments to start the app:

    ./followthestock --help
    usage: followthestock -username toto@gmail.com -password pass
      -dbfile="followthestock.db": database file
      -debug=false: Enable debugging
      -exact=false: Exact timing
      -notls=false: Disable TLS
      -password="SuperStock": XMPP Password
      -server="talk.google.com:443": XMPP Server
      -username="followthestock@gmail.com": XMPP Username
      
The `-exact` option enforces a precise (1 minute) period between calls.

# Client comands

Each client can send the following commands:

* `!help` - Display help
* `!s <stock> <per>` - Subscribe to variation about a stock
* `!u <stock>` - Unsubscribe from a stock
* `!g <stock>` - Get data about a stock
* `!ls` - List currently monitored stocks
* `!v <stock> <nb> <cost>` - Register the cost of our current stocks to calculate the added value
* `!pause <days>` - Pause alerts for X days
* `!resume` - Resume alerts
* `!uptime` - Bot uptime

Here are valid stock formats:
* `RNO` is like `FR:RNO`, which is the french "RENAULT" stock
* `US:RNO` is the "RHINO RESOURCE PARTNERS LP" stock
* `FR0011574110` is like `W:FR0011574110` which is the "SOGEN 50C 0614S" warrant


# Stocks data source
The stocks are fetched from [boursorama](http://www.boursorama.com). It is not an official API, it might not be legal to fetch data and it might not work in the future.
