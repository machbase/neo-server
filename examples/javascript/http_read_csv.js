
q = "select * from example"
fetch(`http://127.0.0.1:5654/db/query?q=${encodeURIComponent(q)}&format=csv`)
  .then(res => {
    return res.text();
  })
  .then(data => {
    console.log(data)
  })
  .catch(err => {
    console.log('Fetch Error', err);
  });