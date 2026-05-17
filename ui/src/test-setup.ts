import '@testing-library/jest-dom'

// jsdom stubs for browser APIs used by TopologyMap
globalThis.ResizeObserver = class {
  observe()    {}
  unobserve()  {}
  disconnect() {}
}

globalThis.requestAnimationFrame = (cb: FrameRequestCallback): number => {
  setTimeout(cb, 16)
  return 1
}
globalThis.cancelAnimationFrame = () => {}
