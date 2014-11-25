#!/usr/bin/env python
# -*- coding: utf-8 -*-

import ConfigParser
import base64
import json
import logging
import traceback

from flask import Flask, Response, request, session, redirect, url_for, render_template
from influxdb import client as influxdb


username = None
password = None
host = None
port = None
user = None
pswd = None
schema = None

# Load configuration
def loadConf():
    global username
    global password
    global host
    global port
    global user
    global pswd
    global schema
    try:
        cfg = ConfigParser.ConfigParser()
        cfg.read('./ss_view.conf')
        username = cfg.get('account', 'username')
        password = cfg.get('account', 'password')
        host = cfg.get('database', 'host')
        port = cfg.get('database', 'port')
        user = cfg.get('database', 'user')
        pswd = cfg.get('database', 'pswd')
        schema = cfg.get('database', 'schema')
        print 'Load conf'
        logging.info('Load conf')
    except:
        traceback.print_exc()
        print 'Load conf error'
        logging.error('Load conf error')

# Web server
app = Flask(__name__, template_folder="templates", static_folder="static")
app.secret_key = 'why would I tell you my secret key?'


@app.route('/')
def index():
    return render_template('index.html')

@app.route('/default')
def default():
    return 'Smart Stock'

@app.route('/login')
def login():
    global username
    global password
    try:
        if session.has_key('user'):
            return json.dumps({'login_result' : 'true'})
        else:
            uname = request.args.get('username')
            pswd = base64.b64encode(request.args.get('password'))
            if (uname == username) and (pswd == password):
                session['user'] = username
                print 'Login true: ' + username
                logging.info('Login true: ' + username)
                return json.dumps({'login_result' : True})
            else:
                print 'Login false'
                logging.info('Login false')
                return json.dumps({'login_result' : False})
    except:
        traceback.print_exc()
        print 'Login error'
        logging.error('Login error')

@app.route('/logout')
def logout():
    try:
        if session.has_key('user'):
            del session['user']
        logging.info('Logout')
        return json.dumps({'logout' : 'true'})
    except:
        traceback.print_exc()
        print 'Logout error'
        logging.error('Logout error')

@app.route('/alert/<exchange>/<date>/<time>')
def alert(exchange, date, time):
    global host
    global port
    global user
    global pswd
    global schema
    try:
        if not session.has_key('user'):
            return "{}", 403
        else:
            db = influxdb.InfluxDBClient(host, int(port), user, pswd, schema)
            result = db.query("select * from alert where dataDate = '%s' and dataTime > '%s'" % (date, time))
            return json.dumps(result)
    except:
        traceback.print_exc()
        print 'Alert error'
        logging.error('Alert error')

@app.route('/report/<exchange>/<date>')
def report(exchange, date):
    global host
    global port
    global user
    global pswd
    global schema
    try:
        if not session.has_key('user'):
            return "{}", 403
        else:
            db = influxdb.InfluxDBClient(host, int(port), user, pswd, schema)
            result = db.query("select * from alert where dataDate = '%s'" % (date))
            def generate():
                cols = result[0]['columns']
                datas = result[0]['points']
                yield '%s,%s,%s,%s,%s,%s' % (cols[0],cols[1],cols[2],cols[3],cols[4],cols[5])
                yield '\n'
                for data in datas:
                    yield '%s,%s,%s,%s,%s,%s' % (data[0],data[1],data[2],data[3],data[4],data[5])
                    yield '\n'
            return Response(generate(), mimetype='text/csv')
    except:
        traceback.print_exc()
        print 'Report error'
        logging.error('Report error')

# Main function
if __name__ == '__main__':
    # Initialize logging
    logging.basicConfig(filename='./ss_view.log', level=logging.INFO, filemode='a', format='%(asctime)s - %(levelname)s: %(message)s')
    # Load configuration
    loadConf()
    # Loop server
    print 'Run server'
    logging.info('Run server')
    app.run(debug=True)




