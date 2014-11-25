from influxdb import client as influxdb
from datetime import datetime
from datetime import timedelta
import json
import time

db = influxdb.InfluxDBClient("10.211.55.4", 8086, "root", "root", "smartstock")
data = {}
data["name"] = "alert"
data["columns"] = ["ticker.exchange", "dataDate", "dataTime", "criteriaHit"]
data["points"] = [["", "", "", ""]]
body = [data]

d = datetime(2014, 11, 25, 23, 33, 0)
n = 600000
m = 1
while True:
    data["points"][0][0] = str(n) + ".XSHG"
    data["points"][0][1] = d.strftime("%Y-%m-%d %H:%M:%S").split(" ")[0]
    data["points"][0][2] = d.strftime("%Y-%m-%d %H:%M:%S").split(" ")[1]
    data["points"][0][3] = "c" + str(m)
    n = n + 1
    if n == 600998:
        n = 600000
    m = m + 1
    if m == 3:
        m = 1
    d = d + timedelta(seconds = 58)
    db.write_points(body)
    print json.dumps(body)
    print "------------------------------------------------------------------------------------------------------------"
    time.sleep(3)

