# -*- coding: utf-8 -*-
#!/usr/bin/python

from flask import Flask, request
from flask_restful import Resource, Api, reqparse
from json import dumps

import sqlite3

class Sql():
    def __init__(self, dbName):
        self.conn = sqlite3.connect(dbName)
        self.crsr = self.conn.cursor()
        
        cmd = """CREATE TABLE IF NOT EXISTS domains 
        (domain VARCHAR(80) PRIMARY KEY);"""
        self.crsr.execute(cmd)
        self.conn.commit()
            
    def populateDummyData(self):
        # SQL command to insert the data in the table
        sql_command = """INSERT OR IGNORE INTO domains VALUES ("google.com");"""
        self.crsr.execute(sql_command)
        self.conn.commit()
    
    def getBlockedList(self):
        cmd = "SELECT * FROM domains;"
        self.crsr.execute(cmd)
        return self.crsr.fetchall()
        

class BlockedList(Resource):
    def __init__(self):
        self.sql = Sql("master_blocked_list.db")
        self.sql.populateDummyData()
        
    def get(self):
        print("Request for blocked list made by client: ")
        blocked = self.sql.getBlockedList()
        print(blocked)
        #return [{'Domain':'Earth'}]
        return blocked
    
    def post(self):
        print("Client sent RAPPOR things to us: ")
        data = request.data
        print "Data: ", data[0]
        return '', 200

def main():
    app = Flask(__name__)
    api = Api(app)
    api.add_resource(BlockedList, "/blocked")
    print("Starting Metis server...")
    app.run(debug=True, threaded=True)
    
main()













