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
  })
  .then(res => {
    return res.json();
  })
  .then(data => {
    console.log(data)
  })
  .catch(err => {
    console.log('Fetch Error', err);
  });