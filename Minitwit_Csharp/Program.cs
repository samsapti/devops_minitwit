using Microsoft.Data.Sqlite;
using Microsoft.AspNetCore.Mvc;

var dbPath = @"./tmp/minitwit.db";
var perPage = 30;
var debug = true;
var secretKey = "development key";
var sql = File.ReadAllText("./schema.sql");



var builder = WebApplication.CreateBuilder(args);
var app = builder.Build();

app.MapGet("/", () => "Hello World!");

app.Run();


// open connection to db
SqliteConnection connectDb()
{
    return new SqliteConnection("Filename={dbPath}");
}

// create db tables
void initDb(SqliteConnection connection)
{
    try
    {
        using(connection)
        {
            using (SqliteCommand sqlReader = connection.CreateCommand())
            {
                connection.Open();
                sqlReader.CommandText = sql;
                sqlReader.ExecuteNonQuery();
            }
        }
    }
    finally
    {
        connection.Close();
    }
}


void queryDb(string query, string[] args, bool one = false)
{
    
}

void getUserId(string username)
{
    
}

void format_datetime(DateTime timestamp)
{
    
}

void gravatar_url(string username)
{
    
}







