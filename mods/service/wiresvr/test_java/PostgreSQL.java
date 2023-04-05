import java.sql.*;

public class PostgreSQL {
    public static void main(String[] args) throws ClassNotFoundException {
        Class.forName("org.postgresql.Driver");

        // IMPORTENT!!!!
        // - preferQueryMode=simple, default is 'extended' which is not supported by
        // wiresvr
        //
        String connurl = "jdbc:postgresql://127.0.0.1:5651/postgres?preferQueryMode=simple";

        try (Connection conn = DriverManager.getConnection(connurl);) {
            PreparedStatement stmt = conn.prepareStatement(
                    "SELECT name, time, value FROM example WHERE name = ? ORDER BY time DESC LIMIT ?");
            stmt.setString(1, "wave.sin");
            stmt.setInt(2, 10);
            ResultSet rs = stmt.executeQuery();

            while (rs.next()) {
                String name = rs.getString("name");
                Timestamp ts = rs.getTimestamp("time");
                Double value = rs.getDouble("value");

                System.out.println("> " + name + " " + ts.toString() + " " + value.toString());
            }
            rs.close();
            stmt.close();
            conn.close();
        } catch (SQLException e) {
            e.printStackTrace();
        }
    }
}