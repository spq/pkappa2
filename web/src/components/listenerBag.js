export default class ListenerBag {
  constructor() {
    this.listeners = [];
  }

  addListener(node, event, callback) {
    this.listeners.push({ node, event, callback });
    node.addEventListener(event, callback);
  }

  clear() {
    this.listeners.forEach(({ node, event, callback }) =>
      node.removeEventListener(event, callback)
    );
    this.listeners = [];
  }
}
