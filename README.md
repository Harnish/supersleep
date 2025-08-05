# supersleep - sleep with output

This is a quick tool I use in replace of sleep so I can tell how much longer my sleep command has.  This is not 100% accurate.  It really doesn't matter to me for what I use it for.  

My main use is when I kick off a sleep command for multiple hours and come back later and want to know where it is at.  I could probably add different things but this was straight forward.  Maybe I should make it run like sleep but if you send it a signal it could change the view.  

## Build
```
go build
```

## Run
```
# ./supersleep -t 14m

Time remaining 840 seconds remaining. Refresh Rate 2 seconds

 # ./supersleep -b 1h
   0% |                                                  | (2/1800, 60 it/min) [2s:29m59s]
```
