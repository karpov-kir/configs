import { Logger } from './Logger';

const logger = Logger.root.scope('PubSub');

type HandlerErrorSink<Channel, Data> = (context: { error: unknown; channel: Channel; data: Data }) => void;

type ChannelData<ChannelMap, Channel extends keyof ChannelMap> = ChannelMap[Channel] extends (
  data: infer Payload,
) => void
  ? Payload
  : never;

type AnyChannel<ChannelMap> = keyof ChannelMap;
type AnyData<ChannelMap> = ChannelData<ChannelMap, keyof ChannelMap>;

type PubSubOptions<ChannelMap> = {
  logPublishes?: boolean;
  channelsToLog?: Set<AnyChannel<ChannelMap>>;
  // A throwing subscriber is isolated: reported via onHandlerError while the rest still
  // receive. On by default — without it, one subscriber's throw breaks the publish loop
  // and propagates to the publisher, a footgun.
  isolateHandlerErrors?: boolean;
  // Sink for an isolated handler error, given the channel and data in play. Defaults to
  // logging the channel and error — never the data.
  onHandlerError?: HandlerErrorSink<AnyChannel<ChannelMap>, AnyData<ChannelMap>>;
};

/** A subscriber callback for one channel's payload — e.g. `ChannelHandler<ReviewState>` is `(state: ReviewState) => void`. */
export type ChannelHandler<Data> = (data: Data) => void;

/** Subscribes (or unsubscribes) a handler for one channel's payload — the shape owners expose as domain verbs. */
export type ChannelSubscriber<Data> = (handler: ChannelHandler<Data>) => void;

/** Typed pub/sub keyed by a channel map — many named channels, each with its own payload. */
export class PubSub<ChannelMap extends object> {
  public logPublishes: boolean;
  public channelsToLog: Set<AnyChannel<ChannelMap>>;
  public isolateHandlerErrors: boolean;
  private readonly onHandlerError: HandlerErrorSink<AnyChannel<ChannelMap>, AnyData<ChannelMap>>;

  private readonly handlers: Map<AnyChannel<ChannelMap>, Array<ChannelHandler<AnyData<ChannelMap>>>> = new Map();
  private readonly earlyHandlers: Map<AnyChannel<ChannelMap>, Array<ChannelHandler<AnyData<ChannelMap>>>> = new Map();

  constructor({
    logPublishes = false,
    channelsToLog = new Set(),
    isolateHandlerErrors = true,
    onHandlerError,
  }: PubSubOptions<ChannelMap> = {}) {
    this.logPublishes = logPublishes;
    this.channelsToLog = channelsToLog;
    this.isolateHandlerErrors = isolateHandlerErrors;
    this.onHandlerError =
      onHandlerError ??
      (({ error, channel }) => logger.warn(`A subscriber on channel ${String(channel)} threw and was isolated`, error));
  }

  /** Register a high-priority handler that runs before regular subscribers on the same channel. */
  public intercept<Channel extends keyof ChannelMap>({
    channel,
    handler,
  }: {
    channel: Channel;
    handler: ChannelHandler<ChannelData<ChannelMap, Channel>>;
  }) {
    const handlers = this.earlyHandlers.get(channel) ?? [];
    this.earlyHandlers.set(channel, [...handlers, handler]);
  }

  public subscribe<Channel extends keyof ChannelMap>({
    channel,
    handler,
  }: {
    channel: Channel;
    handler: ChannelHandler<ChannelData<ChannelMap, Channel>>;
  }) {
    const handlers = this.handlers.get(channel) ?? [];
    this.handlers.set(channel, [...handlers, handler]);
  }

  public unsubscribe<Channel extends keyof ChannelMap>({
    channel,
    handler,
  }: {
    channel: Channel;
    handler: ChannelHandler<ChannelData<ChannelMap, Channel>>;
  }) {
    const handlers = this.handlers.get(channel) ?? [];
    this.handlers.set(
      channel,
      handlers.filter((existingHandler) => existingHandler !== handler),
    );

    const earlyHandlers = this.earlyHandlers.get(channel) ?? [];
    this.earlyHandlers.set(
      channel,
      earlyHandlers.filter((existingHandler) => existingHandler !== handler),
    );
  }

  public get listenersCount(): number {
    let count = 0;
    this.handlers.forEach((handlers) => (count += handlers.length));
    this.earlyHandlers.forEach((handlers) => (count += handlers.length));
    return count;
  }

  public unsubscribeAll({ warnOnLeaks = false }: { warnOnLeaks?: boolean } = {}): void {
    if (warnOnLeaks && this.listenersCount > 0) {
      logger.warn(
        `PubSub: unsubscribeAll called with ${this.listenersCount} active listener(s). Possible subscription leak.`,
      );
    }
    this.handlers.clear();
    this.earlyHandlers.clear();
  }

  public publish<Channel extends keyof ChannelMap>({
    channel,
    data,
  }: {
    channel: Channel;
    data: ChannelData<ChannelMap, Channel>;
  }) {
    const earlyHandlers = this.earlyHandlers.get(channel) ?? [];
    const handlers = this.handlers.get(channel) ?? [];

    if (this.logPublishes && (this.channelsToLog.size === 0 || this.channelsToLog.has(channel))) {
      logger.debug(`Publishing on channel ${String(channel)} with data:`, data);
    }

    earlyHandlers.forEach((handler) => this.deliver({ channel, handler, data }));
    handlers.forEach((handler) => this.deliver({ channel, handler, data }));
  }

  private deliver({
    channel,
    handler,
    data,
  }: {
    channel: AnyChannel<ChannelMap>;
    handler: ChannelHandler<AnyData<ChannelMap>>;
    data: AnyData<ChannelMap>;
  }) {
    if (!this.isolateHandlerErrors) {
      handler(data);
      return;
    }
    try {
      handler(data);
    } catch (error) {
      this.onHandlerError({ error, channel, data });
    }
  }
}
