using System.Text.RegularExpressions;
using Microsoft.Data.Sqlite;

namespace Minitwit;
public class Program 
{
    static string dbPath = @"./tmp/minitwit.db";

    static void Main(string[] args)
    {
        var perPage = 30;
        var debug = true;
        var secretKey = "development key";

        var builder = WebApplication.CreateBuilder(args);
        var app = builder.Build();

        app.Run();

        
    }

    // open connection to db
    static SqliteConnection connectDb()
    {
        return new SqliteConnection($"Data Source={dbPath}");
    }

    // create db tables
    static void initDb(SqliteConnection connection)
    {
        try
        {
            using(connection)
            {
                using (SqliteCommand sqlReader = connection.CreateCommand())
                {
                    connection.Open();
                    sqlReader.CommandText = File.ReadAllText("./schema.sql");
                    sqlReader.ExecuteNonQuery();
                }
            }
        }
        finally
        {
            connection.Close();
        }   
    }

    static void initDb2()
    {
        using (SqliteConnection connection = connectDb())
        {
            connection.Open();
            SqliteCommand sqlReader = connection.CreateCommand();
            sqlReader.CommandText = File.ReadAllText("./schema.sql");
            sqlReader.ExecuteNonQuery();
        }
    }

    static object? queryDb(string query, string[] args, bool one = false)
    {
        // Replace the ?s with the args.
        for(var i = 0; i < args.Length; i++)
        {
            var regex = new Regex(Regex.Escape("?"));
            query = regex.Replace(query, args[i], 1);
        }

        using (SqliteConnection connection = connectDb())
        {
            connection.Open();
            SqliteCommand cmd = connection.CreateCommand();
            cmd.CommandText = query;
            
            if(one)
            {
                return cmd.ExecuteScalar();
            }
            else
            {
                
            }
            
        }
    }


    int getUserId(string username)
    {
        using (SqliteConnection connection = connectDb())
        {
            connection.Open();
            SqliteCommand cmd = connection.CreateCommand();
            cmd.CommandText = "select user_id from user where username = '?'";
            cmd.Parameters.AddWithValue("?", username);
            
            Int32 rv = Convert.ToInt32(cmd.ExecuteScalar());

            return rv;
        }
    }

    public string format_datetime(DateTime timestamp)
    {
        var formatted_datetime = timestamp

        return formatted_datetime;
    }

    public void gravatar_url(string username)
    {
        
    }





    

}