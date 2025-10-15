# Machbase Neo HTTP JavaScript Client

## Query

**JSON**

Set `format=json` or omit it for the default.

```js
q = "select * from example"
fetch(`http://127.0.0.1:5654/db/query?q=${encodeURIComponent(q)}`)
  .then(res => {
    return res.json();
  })
  .then(data => {
    console.log(data)
  });
```

**CSV**

Set `format=csv` explicitly.

```js
q = "select * from example"
fetch(`http://127.0.0.1:5654/db/query?q=${encodeURIComponent(q)}&format=csv`)
  .then(res => {
    return res.text();
  })
  .then(data => {
    console.log(data)
  });
```

## Write

**JSON**

```js
payload = {
    data: {
        columns: ["NAME", "TIME", "VALUE"],
        rows: [
            ['temperature',1677033057000000000,21.1],
            ['humidity',1677033057000000000,0.53]
        ]    
    }
}

fetch('http://127.0.0.1:5654/db/write/example', {
    method: 'POST',
    headers: {
        'Content-Type':'application/json'
    },
    body: JSON.stringify(payload)
  });
```

**CSV**

```js
payload = `temperature,1677033057000000000,21.1
humidity,1677033057000000000,0.53`

fetch('http://127.0.0.1:5654/db/write/example', {
    method: 'POST',
    headers: {
        'Content-Type':'text/csv'
    },
    body: payload
  });
```