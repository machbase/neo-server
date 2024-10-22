package com.machbase.neo.example;

import java.time.LocalDateTime;
import java.time.ZoneOffset;
import java.time.format.DateTimeFormatter;
import java.util.ArrayList;
import java.util.List;

import com.google.protobuf.Any;
import com.google.protobuf.DoubleValue;
import com.machbase.neo.rpc.*;
import com.machbase.neo.rpc.MachbaseGrpc.MachbaseBlockingStub;
import com.machbase.neo.rpc.Machrpc.Column;
import com.machbase.neo.rpc.Machrpc.ColumnsResponse;
import com.machbase.neo.rpc.Machrpc.QueryRequest;
import com.machbase.neo.rpc.Machrpc.QueryResponse;
import com.machbase.neo.rpc.Machrpc.RowsFetchResponse;
import com.google.protobuf.Int32Value;
import com.google.protobuf.StringValue;
import com.google.protobuf.Timestamp;

import java.io.InputStream;
import java.io.ByteArrayInputStream;
import java.io.FileInputStream;
import java.security.KeyStore;
import javax.net.ssl.KeyManagerFactory;
import javax.net.ssl.KeyManager;
import java.nio.file.Path;
import java.nio.file.Paths;
import io.grpc.Grpc;
import io.grpc.ChannelCredentials;
import io.grpc.TlsChannelCredentials;
import io.grpc.ManagedChannel;

public class Example {
    public static void main(String[] args) throws Exception {
        String homeDir = System.getProperty("user.home");
        Path serverCert = Paths.get(homeDir, ".config", "machbase", "cert", "machbase_cert.pem");
        // Path clientKey =  Paths.get(homeDir, ".config", "machbase", "cert", "machbase_key.pem");
        // Path clientCert = Paths.get(homeDir, ".config", "machbase", "cert", "machbase_cert.pem");
        Path clientKey =  Paths.get(homeDir, ".config", "machbase", "cert", "cli-rsa_key.pem");
        Path clientCert = Paths.get(homeDir, ".config", "machbase", "cert", "cli-rsa_cert.pem");

        // keytool -importkeystore -srckeystore certificate.p12 -srcstoretype pkcs12 -destkeystore cert.jks
        System.out.println(serverCert.toFile().toPath());
        System.out.println(clientKey.toFile().toPath());
        System.out.println(clientCert.toFile().toPath());

        // InputStream stream = new FileInputStream(clientCert.toFile());
        // KeyStore keyStore = KeyStore.getInstance("PKCS12");
        // keyStore.load(stream, null);

        // KeyManagerFactory kmf = KeyManagerFactory.getInstance(
        //     KeyManagerFactory.getDefaultAlgorithm());
        // kmf.init(keyStore, null);
        // KeyManager[] keyManagers = kmf.getKeyManagers();

        ChannelCredentials creds = TlsChannelCredentials.newBuilder()
            .trustManager(serverCert.toFile())
            .keyManager(clientCert.toFile(), clientKey.toFile())
            .build();
        ManagedChannel channel = Grpc.newChannelBuilderForAddress("127.0.0.1", 5655, creds)
            .overrideAuthority("neo.machbase.com")
            .build();
        MachbaseBlockingStub stub = MachbaseGrpc.newBlockingStub(channel);

        QueryRequest.Builder builder = Machrpc.QueryRequest.newBuilder();
        builder.setSql("select * from example order by time desc limit ?");
        builder.addParams(Any.pack(Int32Value.of(10)));

        QueryRequest req = builder.build();
        QueryResponse rsp = stub.query(req);

        try {
            ColumnsResponse cols = stub.columns(rsp.getRowsHandle());
            ArrayList<String> headers = new ArrayList<String>();
            headers.add("RowNum");
            for (int i = 0; i < cols.getColumnsCount(); i++) {
                Column c = cols.getColumns(i);
                headers.add(c.getName() + "(" + c.getType() + ")");
            }

            int nrow = 0;
            RowsFetchResponse fetch = null;
            while (true) {
                fetch = stub.rowsFetch(rsp.getRowsHandle());
                if (fetch == null || fetch.getHasNoRows()) {
                    break;
                }
                nrow++;
            
                ArrayList<String> line = new ArrayList<String>();
                line.add(Integer.toString(nrow, 10));
                List<Any> row = fetch.getValuesList();
                for (Any anyv : row) {
                    line.add(convpb(anyv));
                }
                System.out.println(String.join("    ", line));
            }
        } finally {
            stub.rowsClose(rsp.getRowsHandle());
            channel.shutdown();
        }
    }

    static DateTimeFormatter sdf = DateTimeFormatter.ofPattern("yyyy.MM.dd HH:mm:ss.SSS");

    static String convpb(Any any) {
        try {
            switch (any.getTypeUrl()) {
                case "type.googleapis.com/google.protobuf.StringValue": {
                    StringValue v = any.unpack(StringValue.class);
                    return v.getValue();
                }
                case "type.googleapis.com/google.protobuf.Timestamp": {
                    Timestamp v = any.unpack(Timestamp.class);
                    LocalDateTime ldt = java.time.LocalDateTime.ofEpochSecond(v.getSeconds(), v.getNanos(), ZoneOffset.UTC);
                    return ldt.format(sdf);
                }
                case "type.googleapis.com/google.protobuf.DoubleValue": {
                    DoubleValue v = any.unpack(DoubleValue.class);
                    return Double.toString(v.getValue());
                }
                default:
                    return "unsupported " + any.getTypeUrl();
            }
        } catch (Exception e) {
            return "error " + e.getMessage();
        }
    }
}