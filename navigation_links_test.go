package slides

import "testing"

func TestNavigationStepLinks(t *testing.T) {
	runNavigationJS(t, navLinkScript()+`
const assert = require('node:assert/strict');
assert.deepEqual(readPosition('#3/17', 5), {index: 2, step: 17});
assert.deepEqual(readPosition('#3', 5), {index: 2, step: 0});
assert.deepEqual(readPosition('#3/0', 5), {index: 2, step: 0});
assert.deepEqual(readPosition('#3present', 5), {index: 2, step: 0});
assert.deepEqual(readPosition('#3/17present', 5), {index: 2, step: 17});
assert.deepEqual(readPosition('#999/4', 5), {index: 4, step: 4});
for (const hash of ['', '#0/2', '#3/-1', '#notes', '#3/1junk', '#3/1/2']) {
  assert.deepEqual(readPosition(hash, 5), {index: 0, step: 0});
}
assert.equal(positionHash(2, 17, false), '#3/17');
assert.equal(positionHash(2, 0, false), '#3');
assert.equal(positionHash(2, 17, true), '#3/17present');
for (const position of [{index: 0, step: 0}, {index: 4, step: 31}]) {
  assert.deepEqual(readPosition(positionHash(position.index, position.step, false), 5), position);
}
`)
}
