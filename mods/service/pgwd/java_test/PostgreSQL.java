import java.sql.*;

public class PostgreSQL {
    public static void main(String[] args) throws ClassNotFoundException {
        Class.forName("org.postgresql.Driver");

        boolean useSimpleQuery = true;
        String connurl = useSimpleQuery ? "jdbc:postgresql://127.0.0.1:5651/postgres?preferQueryMode=simple"
                : "jdbc:postgresql://127.0.0.1:5651/postgres";

        try (Connection conn = DriverManager.getConnection(connurl);) {
            PreparedStatement stmt = conn.prepareStatement(
                    "SELECT name, time, value FROM example WHERE name = ? ORDER BY time DESC LIMIT ?");

            if (useSimpleQuery) {
                stmt.setString(1, "wave.sin");
                stmt.setInt(2, 5);
            } else {
                // pgwire currently support string parameter only
                stmt.setString(1, "wave.sin");
                stmt.setString(2, "5");
            }

            ResultSet rs = stmt.executeQuery();
            while (rs.next()) {
                if (useSimpleQuery) {
                    String name = rs.getString("name");
                    Timestamp ts = rs.getTimestamp("time");
                    Double value = rs.getDouble("value");
                    System.out.println("> " + name + " " + ts.toString() + " " + value.toString());
                } else {
                    String name = rs.getString("name");
                    Timestamp ts = rs.getTimestamp("time");
                    Double value = rs.getDouble("value");
                    System.out.println("> " + name + " " + ts + " " + value);
                }
            }
            rs.close();
            stmt.close();
            conn.close();
        } catch (SQLException e) {
            e.printStackTrace();
        }
    }
}