# LoL Replay

Since this is a personal project, I'll be super lazy to document it. Here's a REALLY quick overview
on how to use it.

LoL Spectator (for OS X) available here: https://github.com/1lann/LoL-Spectator

1. Make an app.conf file, copy it from the app.conf.default file in /conf
2. Put your Riot Games API key in the app.conf file `riotapikey = blahkeyhere`
3. Add platforms you want to record on to `/app/models/record.go` line 15
4. Configure player tracking in `/app/models/monitor.go`. You'll need to modify the players tracked arrays, and the `runMonitor()` function on line 98 ish
5. `revel run replay`
7. Play on an account that is tracked
6. Navigate to http://127.0.0.1:9000 and replays will show up there. Highest is most recent.
7. Profit???!?

Need more help? Contact me on GitHub, mail (1lanncontact@gmail.com), or reddit (/u/1lann) and I'll be happy to help you :)
