type Listeners = Array<{
  node: Node;
  event: string;
  callback: EventListenerOrEventListenerObject;
}>;

export default class ListenerBag {
  listeners: Listeners;

  constructor() {
    this.listeners = [];
  }

  addListener(
    node: Node,
    event: string,
    callback: EventListenerOrEventListenerObject,
  ) {
    this.listeners.push({ node, event, callback });
    node.addEventListener(event, callback);
  }

  clear() {
    this.listeners.forEach(({ node, event, callback }) =>
      node.removeEventListener(event, callback),
    );
    this.listeners = [];
  }
}
