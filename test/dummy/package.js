Package.describe({
  name: 'mtest:dummy',
  version: '0.0.1',
  summary: 'Dummy Meteor package for exercising the mtest CLI.',
  git: 'https://github.com/illusionfield/mtest.git',
  documentation: 'README.md'
});

Package.onUse(api => {
  api.versionsFrom(['2.10.0', '3.0.1', '3.3.2']);
  api.use('ecmascript');
  api.mainModule('dummy.js');
});

Package.onTest(api => {
  api.use([
    'tinytest',
    'ecmascript',
    'mtest:dummy',
  ]);

  api.mainModule('dummy_tests_client.js', 'client');
  api.mainModule('dummy_tests_server.js', 'server');
});
