# -*- coding: utf-8 -*-
#!/usr/bin/python

from flask import Flask, request
from flask_restful import Resource, Api, reqparse

import json
import sqlite3

import rappor_analysis as rappor

class Sql():
    def __init__(self, dbName):
        self.conn = sqlite3.connect(dbName)
        self.crsr = self.conn.cursor()
        
        cmd = """CREATE TABLE IF NOT EXISTS domains 
        (domain VARCHAR(80) PRIMARY KEY);"""
        self.crsr.execute(cmd)
        self.conn.commit()
            
    def populateDummyData(self):
        sql_command = """INSERT OR IGNORE INTO domains VALUES ("google.com");"""
        self.crsr.execute(sql_command)
        sql_command = """INSERT OR IGNORE INTO domains VALUES ("facebook.com");"""
        self.crsr.execute(sql_command)
        self.conn.commit()
        
    def addToDB(self, domain):
        cmd = "INSERT OR IGNORE INTO domains VALUES (\""+domain+"\");"
        self.crsr.execute(cmd)
        self.conn.commit()
    
    def getBlockedList(self):
        cmd = "SELECT * FROM domains;"
        self.crsr.execute(cmd)
        return self.crsr.fetchall()
    
    def clearDB(self):
        cmd = "DELETE FROM domains;"
        self.crsr.execute(cmd)
        self.conn.commit()
        

class BlockedList(Resource):
    def __init__(self):
        self.sql = Sql("master_blocked_list.db")
        #self.sql.clearDB()
        self.sql.populateDummyData()
        
    def get(self):
        print("Request for blocked list made by client: ")
        sqlBlocked = self.sql.getBlockedList()
        blocked = []
        for dom in sqlBlocked:
            blocked.append({'Domain':dom[0]})
        return blocked
    
    def post(self):
        print("Client sent RAPPOR things to us: ")
        data = request.data
        print(data)
        cliMsg = json.loads(data)
        print("Client message is ", cliMsg)
        reps = {cliMsg["cohort"]:cliMsg["reports"]}
        numDomains = 10
        params = rappor.Params(prob_f=0.2)
        doms = rappor.analyzeReports(reps, params, numDomains)
        for d in doms:
            self.sql.addToDB(d)
        return '', 200

def main():
    app = Flask(__name__)
    api = Api(app)
    api.add_resource(BlockedList, "/blocked")
    print("Starting Metis server...")
    app.run(debug=True, threaded=True)
    
main()













