// Runs the callback page script against a minimal fake browser and prints the
// observable state as JSON. Usage: node auth_page_harness.js <scenario> < script.js
// Scenarios: pending, success, http-error, network-error.
const vm = require("vm");

const scenario = process.argv[2];
const script = require("fs").readFileSync(0, "utf8");

function makeElement(tag) {
  let text = "";
  const el = {
    tagName: tag,
    children: [],
    href: "",
    appendChild(child) {
      if (typeof child !== "object" || child === null) {
        throw new TypeError("appendChild: argument is not a Node");
      }
      this.children.push(child);
      return child;
    },
  };
  // Like the DOM, setting textContent drops the existing children.
  Object.defineProperty(el, "textContent", {
    get: () => text,
    set: (v) => {
      text = String(v);
      el.children = [];
    },
  });
  return el;
}

const statusElement = makeElement("p");
statusElement.textContent = "Authenticating...";

const requests = [];
const timers = [];
const navigations = [];
let windowStatus = "";

class FakeXHR {
  constructor() {
    this.status = 0;
    this.responseText = "";
    requests.push(this);
  }
  open(method, url, async) {
    this.method = method;
    this.url = url;
    this.async = async;
  }
  send(body) {
    this.sent = true;
    this.body = body;
  }
}

// The page runs as a top-level script, so its `var` declarations land on the
// global object, which is the window. In browsers window.status is a string
// accessor: assigning an element to it stores the element's string form.
const sandbox = {
  URLSearchParams,
  encodeURIComponent,
  XMLHttpRequest: FakeXHR,
  document: {
    getElementById: (id) => (id === "status" ? statusElement : null),
    createElement: makeElement,
  },
  setTimeout: (fn, delay) => timers.push({ fn, delay }),
};
sandbox.window = sandbox;
Object.defineProperty(sandbox, "status", {
  get: () => windowStatus,
  set: (v) => {
    windowStatus = String(v);
  },
});
let location = { search: "?code=abc%20123" };
Object.defineProperty(sandbox, "location", {
  get: () => location,
  set: (v) => {
    navigations.push(String(v));
  },
});
vm.createContext(sandbox);

function snapshot() {
  return {
    text: statusElement.textContent,
    children: statusElement.children.map((c) => ({ tag: c.tagName, href: c.href, text: c.textContent })),
    requests: requests.map((r) => ({ method: r.method, url: r.url, async: r.async, sent: !!r.sent })),
    timers: timers.map((t) => t.delay),
    navigations: navigations.slice(),
  };
}

const out = {};
try {
  vm.runInContext(script, sandbox);
  out.pending = snapshot();

  const xhr = requests[0];
  if (scenario === "success") {
    xhr.status = 200;
    xhr.responseText = "Authentication successful, you'll be redirected. Click: ";
    xhr.onload();
  } else if (scenario === "http-error") {
    xhr.status = 401;
    xhr.responseText = "Authentication failed, run 'qovery auth' again.";
    xhr.onload();
  } else if (scenario === "network-error") {
    xhr.onerror();
  }
  out.afterResponse = snapshot();

  // Firing the scheduled redirect must navigate the window.
  timers.forEach((t) => t.fn());
  out.afterTimers = snapshot();
} catch (e) {
  out.error = String(e);
}
console.log(JSON.stringify(out));
