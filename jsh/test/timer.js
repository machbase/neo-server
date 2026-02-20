
const {now} = require('process');

function printNow(count) {
    console.println(`${count} timer => ${now()}`);
    if (count < 3) {
        setTimeout(printNow, 1000, count+1);
    }
}

setTimeout(printNow, 1000, 1)
