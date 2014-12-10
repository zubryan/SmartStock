from influxdb import client as influxdb
from datetime import datetime
from datetime import timedelta
import json
import time

db = influxdb.InfluxDBClient("10.22.129.55", 8086, "root", "root", "smartstock")
data = {}
data["name"] = "alerts"
data["columns"] = ["ticker.exchange", "dataDate", "dataTime", "criteriaHit"]
data["points"] = [["", "", "", ""]]
body = [data]

d = datetime(2014, 12, 8, 00, 16, 26)
n = 600000
m = 1
while True:
    data["points"][0][0] = str(n) + ".XSHG"
    data["points"][0][1] = d.strftime("%Y-%m-%d %H:%M:%S").split(" ")[0]
    data["points"][0][2] = d.strftime("%Y-%m-%d %H:%M:%S").split(" ")[1]
    data["points"][0][3] = "Criteria 1 X11:1.53 X12:2.01 X2:3.39 X3:0.045 X4:303759 Y1:true Y2:false Prc:10.36 Vol:9840212 MA5:10.128 MA10:9.958 MA20:9.938"
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

