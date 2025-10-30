import { Tinytest } from 'meteor/tinytest';
import { meaningOfLife } from 'meteor/mtest:dummy';

Tinytest.add('mtest:dummy - meaningOfLife returns 42', (test) => {
  const div = document.createElement('div');
  document.body.appendChild(div);

  try {
    div.textContent = meaningOfLife();
    test.equal(parseInt(div.textContent, 10), 42, 'meaningOfLife() should resolve to the canonical answer');
  } finally {
    document.body.removeChild(div);
  }
});
