acurite_to_graph
================

Graph your temperature & humidity sensors (AcuRite 06002RM, AcuRite 06044M ...).

It parses the JSON output from `rtl_443` command and expose it to Prometheus.

You can pass a flag to add a `name` label matching a specific id like:

```
acurite_to_graph -nameFields 8831=bedroom,15466=livingroom -debug
2016/10/27 17:45:38 {Acurite tower sensor 15466 A 17.5 52 OK livingroom}
2016/10/27 17:45:41 {Acurite tower sensor 8831 B 17.1 54 OK bedroom}
```

A mini website with your live temperatures is also accessible at http://localhost:44010 

![example webpage](https://github.com/akhenakh/acurite_to_graph/raw/master/img/web.png)

Here is a [blogpost](http://blog.nobugware.com/post/2017/Hacking_temperature_radio_sensors_and_graphing_with_prometheus/) about it.
