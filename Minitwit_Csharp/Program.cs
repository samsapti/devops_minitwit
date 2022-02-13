using System.Text.RegularExpressions;
using System.Security.Cryptography;

using Microsoft.Data.Sqlite;
using System.Text;
using System.Collections.Generic;

//using static System.Net.Mime.MediaTypeNames;

namespace Minitwit;
public class Program 
{
    static string dbPath = @"./tmp/minitwit.db";
    static int perPage = 30;
    static bool debug = true;
    static string secretKey = "development key";

    static void Main(string[] args)
    {
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
                var list = new List<Dictionary<string, string>>();
                using var reader = cmd.ExecuteReader();
                while(reader.Read())
                {
                    var dict = new Dictionary<string, string>();
                    dict.Add("k", "v");
                    list.Add(dict);
                }

                return list;
            }
        }
    }


    int getUserId(string username)
    {
        using (SqliteConnection connection = connectDb())
        {
            connection.Open();
            SqliteCommand cmd = connection.CreateCommand();
            cmd.CommandText = "select user_id from user where username = @?";
            cmd.Parameters.AddWithValue("@?", username);
            
            Int32 rv = Convert.ToInt32(cmd.ExecuteScalar());

            return rv;
        }
    }

    public string format_datetime(DateTime timestamp)
    {
        return timestamp.ToString("yyyy-MM-dd @ hh:mm");
    }

    // TODO : Figure out how to return an image from the hashed link... Returntype? Filetype? 
    // Get profile image by hashing user email and entering the hash into
    public static string gravatar_url(string email, int size = 80)
    {
        MD5 md5Hasher = MD5.Create();
        byte[] data = md5Hasher.ComputeHash(Encoding.Default.GetBytes(email));

        StringBuilder strBuilder = new StringBuilder();

        for(int i = 0; i < data.Length; i++)
        {
            strBuilder.Append(data[i].ToString("x2"));
        }

        string imageString = strBuilder.ToString();

        // return image associated with link...
        
        
        return string.Format("http://www.gravatar.com/avatar/%s?d=identicon&s=%d", imageString, size);
    }

    void timeline()
    {
        
    }

    void public_timeline()
    {
        
    }
    void user_timeline(string username)
    {
        
    }

    void follow_user()
    {
        
    }

    void unfollow_user()
    {
        
    }

    void add_message()
    {
        
    }

    void login()
    {
        
    }

    void register()
    {
        
    }

    void logout()
    {
        
    }


}