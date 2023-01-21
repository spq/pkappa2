import { TypedEmitter } from "tiny-typed-emitter";

const emitter = new TypedEmitter();
export const EventBus = {
    $on: (event: any, listener: (...args: any[]) => any) => emitter.on(event, listener),
    $once: (event: any, listener: (...args: any[]) => any)  => emitter.once(event, listener),
    $off: (event: any, listener: (...args: any[]) => any)  => emitter.off(event, listener),
    $emit: (event: any, listener: (...args: any[]) => any)  => emitter.emit(event, listener)
};
