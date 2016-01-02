# Follow the stock

It follows some stocks on [boursorama](http://www.boursorama.com) and send some alerts through XMPP when it detects some changes over the subscribed percentage threshold.

# Command line interface

Here are the command line interfaces arguments to start the app:

    usage: followthestock -config <file>
      -config="/etc/followthestock/followthestock.conf": Config file
      -console=false: Use console
      -show-config=false: Show config

# Config file

The config file looks something like that:

    [general]
    exactTiming = false
    
    [xmpp]
    username = <username>
    password = <password>
    server = talk.google.com:443
    notls = false
    debug = true
    activityWatchdogMinutes = 30

    [db]
    # In the current working directory (should be /var/lib/followthestock)
    file = followthestock.db

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

# Debian packages
Debian packages are automatically generated here:
 http://94.23.55.152/followthestock/dist/package/
