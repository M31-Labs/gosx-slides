// Optional test tooling only. Production serving and captures do not use Node.
const playwright = require(process.env.SLIDES_PLAYWRIGHT_MODULE || 'playwright');

function launchTestBrowser(options = {}) {
  const engine = process.env.SLIDES_TEST_ENGINE || 'chromium';
  if (!['chromium', 'firefox', 'webkit'].includes(engine)) throw Error('Unknown browser engine: ' + engine);
  const settings = {...options};
  if (engine !== 'chromium') {
    delete settings.args;
    delete settings.executablePath;
  }
  const executable = engine === 'chromium' ? process.env.SLIDES_BROWSER : process.env.SLIDES_TEST_EXECUTABLE;
  if (executable) settings.executablePath = executable;
  return playwright[engine].launch(settings);
}

module.exports = {launchTestBrowser};
