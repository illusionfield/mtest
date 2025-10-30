import { Tinytest } from 'meteor/tinytest';
import { greet } from 'meteor/mtest:dummy';

Tinytest.add('mtest:dummy - greet formats name', (test) => {
  test.equal(greet('User'), 'Hello, User!');
  test.equal(greet(), 'Hello, friend!');
});
