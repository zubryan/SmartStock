#!/usr/bin/env python
# -*- coding: utf-8 -*-

import ConfigParser
import base64
import json
import logging
import os
import platform
import traceback

from flask import Flask, Response, request, session, redirect, url_for, render_template
from influxdb import client as influxdb
import xlwt


username = None
password = None
host = None
port = None
user = None
pswd = None
schema = None
reportdir = None
reportcmd = None

col_names = {'ticker.exchange':'股票代码.交易行', 'dataDate':'选股日期', 'dataTime':'选股时间', 'criteriaHit':'选股规则', 'sequence_number':'记录号', 'time':'记录时间'}

# Load configuration
def loadConf():
    global username
    global password
    global host
    global port
    global user
    global pswd
    global schema
    global reportdir
    global reportcmd
    try:
        cfg = ConfigParser.ConfigParser()
        cfg.read('/opt/SmartStockView/ss_view.conf')
        username = cfg.get('account', 'username')
        password = cfg.get('account', 'password')
        host = cfg.get('database', 'host')
        port = cfg.get('database', 'port')
        user = cfg.get('database', 'user')
        pswd = cfg.get('database', 'pswd')
        schema = cfg.get('database', 'schema')
        reportdir = cfg.get('service', 'reportdir')
        reportcmd = cfg.get('service', 'reportcmd')
        print 'Load conf'
        logging.info('Load conf')
    except:
        traceback.print_exc()
        print 'Load conf error'
        logging.error('Load conf error')

# Web server
app = Flask(__name__, template_folder='templates', static_folder='static')
app.secret_key = 'why would I tell you my secret key?'


@app.route('/')
def index():
    return render_template('index.html')

@app.route('/ping')
def ping():
    return 'Smart Stock Ping'

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
        return redirect('/')
    except:
        traceback.print_exc()
        print 'Logout error'
        logging.error('Logout error')

def initCriteria():
    global host
    global port
    global user
    global pswd
    global schema
    try:
        criterias = [{
                              'name': 'criteria',
                              'columns': ['criteria'],
                              'points': [['criteria1:X1_2 > 2,X2 > 3,Y1 = true,X3 < 0.2,X4 < 500000|criteria2:X1_1 > 2,X2 < -3,Y1 = false,Y2 = false,X3 < 0.2,X4 < 500000']]
                              }]
        try:
            if os.path.exists('/opt/SmartStockView/ss_view.criterias'):
                cf = open('/opt/SmartStockView/ss_view.criterias', 'r')
                cs = eval(cf.readline())
                if (cs != None) and (len(cs) > 0):
                    logging.info('Loaded criteria: %s' % (str(cs)))
                    criterias = cs
        except:
            traceback.print_exc()
            print 'Loaded criteria error'
            criterias = [{
                              'name': 'criteria',
                              'columns': ['criteria'],
                              'points': [['criteria1:X1_2 > 2,X2 > 3,Y1 = true,X3 < 0.2,X4 < 500000|criteria2:X1_1 > 2,X2 < -3,Y1 = false,Y2 = false,X3 < 0.2,X4 < 500000']]
                              }]
        init = False
        try:
            db = influxdb.InfluxDBClient(host, int(port), user, pswd, schema)
            result = db.query("select criteria from criteria limit 1")
            init = False
        except:
            logging.info('Need initialize criteria data')
            init = True
        if init:
            db = influxdb.InfluxDBClient(host, int(port), user, pswd, schema)
            db.write_points(criterias)
            logging.info('Initialize criteria')
        else:
            logging.info('Not need initialize criteria')
    except:
        traceback.print_exc()
        print 'Initialize criteria error'
        logging.error('Initialize criteria error')  

def last_criteria():
    global host
    global port
    global user
    global pswd
    global schema
    try:
        db = influxdb.InfluxDBClient(host, int(port), user, pswd, schema)
        result = db.query("select criteria from criteria limit 1")
        criterias = result[0]['points'][0][2].split('|')
        logging.info('Last criteria: ' + result[0]['points'][0][2])
        criteria_dict = {}
        for criteria in criterias:
            criteria = criteria.strip()
            criteria_name = criteria.split(':')[0].strip()
            criteria_bodys = criteria.split(':')[1].strip().split(',')
            criteria_dict[criteria_name] = []
            for criteria_body in criteria_bodys:
                criteria_node = []
                criteria_items = criteria_body.split(' ')
                for criteria_item in criteria_items:
                    if len(criteria_item.strip()) > 0:
                        criteria_node.append(criteria_item.strip())
                criteria_dict[criteria_name].append(criteria_node)
        return json.dumps(criteria_dict)
    except:
        traceback.print_exc()
        print 'Last criteria error'
        logging.error('Last criteria error')
        return '{}'

def build_criteria(data):
    global host
    global port
    global user
    global pswd
    global schema
    try:
        db = influxdb.InfluxDBClient(host, int(port), user, pswd, schema)
        criterias = [{
                              'name': 'criteria',
                              'columns': ['criteria'],
                              'points': [[]]
                              }]
        if data != None:
            criteria_dict = json.loads(data)
            criteria_list = []
            for key in criteria_dict.iterkeys():
                criteria_list.append(key)
                criteria_list.append(':')
                criteria_items = criteria_dict[key]
                for criteria_item in criteria_items:
                    for item in criteria_item:
                        criteria_list.append(item)
                        criteria_list.append(' ')
                    del criteria_list[len(criteria_list) - 1]
                    criteria_list.append(',')
                del criteria_list[len(criteria_list) - 1]
                criteria_list.append('|')
            del criteria_list[len(criteria_list) - 1]
            criterias[0]['points'][0].append(''.join(criteria_list))
            db.write_points(criterias)
            try:
                cf = open('/opt/SmartStockView/ss_view.criterias', 'w')
                cf.write(str(criterias))
                cf.flush()
                cf.close()
                logging.info('Saved criteria: %s' % (str(criterias)))
            except:
                traceback.print_exc()
                print 'Saved criteria error'
            logging.info('Build criteria: ' + ''.join(criteria_list))
            return last_criteria()
        else:
            return '{}'
    except:
        traceback.print_exc()
        print 'Build criteria error'
        logging.error('Build criteria error')
        return '{}'
    

@app.route('/criteria', methods=['GET', 'POST'])
def criteria():
    global host
    global port
    global user
    global pswd
    global schema
    if not session.has_key('user'):
            return '{}', 403
    else:
        try:
            db = influxdb.InfluxDBClient(host, int(port), user, pswd, schema)
            if request.method == 'GET':
                return last_criteria()
            else:
                return build_criteria(request.data)
        except:
            traceback.print_exc()
            print 'Criteria error'
            logging.error('Criteria error')

@app.route('/alert/<exchange>/<date>/<time>')
def alert(exchange, date, time):
    global host
    global port
    global user
    global pswd
    global schema
    try:
        if not session.has_key('user'):
            return '{}', 403
        else:
            db = influxdb.InfluxDBClient(host, int(port), user, pswd, schema)
            result = db.query(''' select * from "alerts.%s" where dataTime > '%s' ''' % (date, time))
            logging.info(''' select * from "alerts.%s" where dataTime > '%s' ''' % (date, time))
            return json.dumps(result)
    except:
        traceback.print_exc()
        print 'Alert error'
        logging.error('Alert error')
        return ''' [{"points": [], "name": "alerts.empty", "columns": ["time", "sequence_number", "ticker.exchange", "dataDate", "dataTime", "criteriaHit"]}] '''

@app.route('/report/<exchange>/<date>.xls')
def report(exchange, date):
    global host
    global port
    global user
    global pswd
    global schema
    global col_names
    global reportdir
    global reportcmd
    try:
        if not session.has_key('user'):
            return '{}', 403
        else:
            criterias = None
            suffix = None
            try:
                db = influxdb.InfluxDBClient(host, int(port), user, pswd, schema)
                result = db.query("select criteria from criteria limit 1")
                criterias = result[0]['points'][0][2].strip()
                suffix = base64.b64encode(criterias)
                if os.path.exists('/opt/SmartStockView/static/report/' + str(date) + '_' + suffix + '.xls'):
                    return redirect('/static/report/' + str(date) + '_' + suffix + '.xls')
            except:
                traceback.print_exc()
                print 'Report error'
                logging.error('Report error')
                return '<html><head><title>报告错误！</title></head><body><h1>报告错误！</h1></body></html>'
            try:
                reportop = '&'
                if platform.system().strip() == 'Linux':
                    reportop = ';'
                reportcmdtag = os.system('%s %s %s %s' % (reportdir, reportop, reportcmd, date))
                logging.info("Report service: %s %s %s %s" % (reportdir, reportop, reportcmd, date))
                logging.info("Report service: %d" % (reportcmdtag))
            except:
                logging.error('Report service error: ' + '%s %s' % (reportcmd, date))
                traceback.print_exc()
            wb = xlwt.Workbook(encoding='utf-8')
            sheet = wb.add_sheet('围数资本选股报告：' + str(date), cell_overwrite_ok=True)
            sheet.col(0).width = 10000
            sheet.col(1).width = 10000
            sheet.col(2).width = 10000
            sheet.col(2).hidden = True
            sheet.col(3).width = 10000
            sheet.col(4).width = 50000
            sheet.col(5).width = 10000
            sheet.col(5).hidden = True
            db = influxdb.InfluxDBClient(host, int(port), user, pswd, schema)
            result = db.query(''' select * from "alerts.%s" where dataDate = '%s' ''' % (date, date))
            if (result == None) or (len(result) < 1):
                sheet.write(0, 0, col_names.values()[0])
                sheet.write(0, 1, col_names.values()[1])
                sheet.write(0, 2, col_names.values()[2])
                sheet.write(0, 3, col_names.values()[3])
                sheet.write(0, 4, col_names.values()[4])
                sheet.write(0, 5, col_names.values()[5])
                sheet.flush_row_data()
            else:
                cols = result[0]['columns']
                datas = result[0]['points']
                col_seqs = ''
                col_keys = col_names.keys()
                for ca in xrange(0, len(col_keys)):
                    for cb in xrange(0, len(cols)):
                        if cols[cb] == col_keys[ca]:
                            col_seqs = col_seqs + str(cb)
                            break
                sheet.write(0, 0, col_names.values()[0])
                sheet.write(0, 1, col_names.values()[1])
                sheet.write(0, 2, col_names.values()[2])
                sheet.write(0, 3, col_names.values()[3])
                sheet.write(0, 4, col_names.values()[4])
                sheet.write(0, 5, col_names.values()[5])
                row = 1
                for data in datas:
                    sheet.write(row, 0, data[int(col_seqs[0])])
                    sheet.write(row, 1, data[int(col_seqs[1])])
                    sheet.write(row, 2, data[int(col_seqs[2])])
                    sheet.write(row, 3, data[int(col_seqs[3])])
                    sheet.write(row, 4, data[int(col_seqs[4])])
                    sheet.write(row, 5, data[int(col_seqs[5])])
                    row = row + 1
                sheet.flush_row_data()
            wb.save('/opt/SmartStockView/static/report/' + str(date) + '_' + suffix + '.xls')
            return redirect('/static/report/' + str(date) + '_' + suffix + '.xls')
    except:
        traceback.print_exc()
        print 'Report error'
        logging.error('Report error')
        return '<html><head><title>报告不存在！</title></head><body><h1>报告不存在！</h1></body></html>'

# Main function
if __name__ == '__main__':
    # Initialize logging
    logging.basicConfig(filename='/datayes/log/ss_view.log', level=logging.INFO, filemode='a', format='%(asctime)s - %(levelname)s: %(message)s')
    # Load configuration
    loadConf()
    # Initialize criteria
    initCriteria()
    # Loop server
    print 'Run server'
    logging.info('Run server')
    app.run(host='0.0.0.0', port=80)
