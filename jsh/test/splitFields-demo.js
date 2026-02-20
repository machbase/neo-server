const {splitFields} = require('util');

console.println('\n=== Test 1: Basic splitting ===');
const result1 = splitFields("hello 'world foo' bar");
console.println(result1); // Expected output: ['hello', 'world foo', 'bar']

console.println('\n=== Test 2: Double quotes ===');
const result2 = splitFields('a "b c" d "e f"');
console.println(result2); // Expected output: ['a', 'b c', 'd', 'e f']

console.println('\n=== Test 3: Mixed quotes and spaces ===');
const result3 = splitFields("one 'two three' four \"five six\" seven");
console.println(result3); // Expected output: ['one', 'two three', 'four', 'five six', 'seven']

console.println('\n=== Test 4: No quotes ===');
const result4 = splitFields("just some simple text");
console.println(result4); // Expected output: ['just', 'some', 'simple', 'text']

console.println('\n=== Test 5: Empty string ===');
const result5 = splitFields("");
console.println(result5); // Expected output: []

console.println('\n=== Test 6: Only spaces ===');
const result6 = splitFields("     ");
console.println(result6); // Expected output: []

console.println('\n=== Test 7: Unmatched quotes ===');
const result7 = splitFields("start 'middle end");
console.println(result7); // Expected output: ['start', 'middle end']

console.println('\n=== Test 8: Nested quotes (treated literally) ===');
const result8 = splitFields('she said "it\'s a test" today');
console.println(result8); // Expected output: ['she', 'said', "it's a test", 'today']

console.println('\n=== Test 9: Tabs and newlines as whitespace ===');
const result9 = splitFields("line1\tline2\n'line 3'\rline4");
console.println(result9); // Expected output: ['line1', 'line2', 'line 3', 'line4']

console.println('\n=== Test 10: Leading and trailing spaces ===');
const result10 = splitFields("   leading and trailing spaces   ");
console.println(result10); // Expected output: ['leading', 'and', 'trailing', 'spaces']

console.println('\n=== Test 11: Multiple consecutive spaces ===');
const result11 = splitFields("multiple    spaces   here");
console.println(result11); // Expected output: ['multiple', 'spaces', 'here']

console.println('\n=== Test 12: Quotes with spaces only ===');
const result12 = splitFields("'   ' \"   \"");
console.println(result12); // Expected output: ['   ', '   ']

console.println('\n=== Test 13: Special characters inside quotes ===');
const result13 = splitFields("'!@#$%^&*()' \"<>?:{}|\"");
console.println(result13); // Expected output: ['!@#$%^&*()', '<>?:{}|']

console.println('\n=== Test 14: Mixed single and double quotes ===');
const result14 = splitFields("'single \"double\" inside' \"double 'single' inside\"");
console.println(result14); // Expected output: ['single "double" inside', "double 'single' inside"]

console.println('\n=== Test 15: Escaped quotes (treated literally) ===');
const result15 = splitFields("He said \\'hello\\' and she replied \\\"hi\\\"");
console.println(result15); // Expected output: ["He", "said", "\\'hello\\'", "and", "she", "replied", '\\"hi\\"']

console.println('\n=== All tests completed ===');
