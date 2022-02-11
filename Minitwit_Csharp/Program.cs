using Microsoft.Data.Sqlite;

namespace Minitwit;
public class Program 
{
    static void Main(string[] args)
    {
        var dbPath = @"./tmp/minitwit.db";
        var perPage = 30;
        var debug = true;
        var secretKey = "development key";


        var builder = WebApplication.CreateBuilder(args);
        var app = builder.Build();

        app.Run();


        // open connection to db
        SqliteConnection connectDb()
        {
            return new SqliteConnection($"Data Source={dbPath}");
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

        void initDb2()
        {
            using (SqliteConnection connection = connectDb())
            {
                connection.Open();
                SqliteCommand sqlReader = connection.CreateCommand();
                sqlReader.CommandText = File.ReadAllText("./schema.sql");
                sqlReader.ExecuteNonQuery();
            }
        }
    }

    object? queryDb(string query, string[] args, bool one = false)
    {
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