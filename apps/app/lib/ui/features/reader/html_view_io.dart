import 'dart:collection';
import 'dart:convert';
import 'dart:io';
import 'package:flutter/material.dart';
import 'package:webview_flutter/webview_flutter.dart';
import 'package:webview_flutter_android/webview_flutter_android.dart';
import '../../../platform/qr_scanner_service.dart';
import 'html_view_models.dart';
import 'html_view_stub.dart' as stub;

export 'html_view_models.dart';

const int _maxLiveWebViewControllers = 2;
final LinkedHashMap<String, _ManagedWebViewEntry> _liveControllers =
    LinkedHashMap<String, _ManagedWebViewEntry>();

ReadalongFrameHighlight? _lastHighlight;

bool shouldUseHtmlViewStubForCurrentEnvironment() {
  if (Platform.environment.containsKey('FLUTTER_TEST')) {
    return true;
  }
  if (!Platform.isAndroid && !Platform.isIOS) {
    return true;
  }
  if (WebViewPlatform.instance == null) {
    return true;
  }
  return false;
}

bool get requiresNativeMediaCookie =>
    !shouldUseHtmlViewStubForCurrentEnvironment();

ReadalongFrameHighlight? get lastReadalongHighlight =>
    shouldUseHtmlViewStubForCurrentEnvironment()
        ? stub.lastReadalongHighlight
        : _lastHighlight;

String activeFragmentForViewKey(String viewKey) {
  if (shouldUseHtmlViewStubForCurrentEnvironment()) {
    return stub.activeFragmentForViewKey(viewKey);
  }
  final hl = _lastHighlight;
  if (hl == null || hl.fragmentId.isEmpty) return '';
  if (hl.targetViewKey != null && hl.targetViewKey != viewKey) return '';
  return hl.fragmentId;
}

void resetHtmlFrameHighlightsForTest() {
  _lastHighlight = null;
  stub.resetHtmlFrameHighlightsForTest();
}

Future<void> seedHtmlAuthCookie({
  required String serverBaseUrl,
  required String mediaJwt,
}) async {
  if (shouldUseHtmlViewStubForCurrentEnvironment()) {
    return;
  }
  final trimmedBase = serverBaseUrl.trim();
  final trimmedJwt = mediaJwt.trim();
  if (trimmedBase.isEmpty || trimmedJwt.isEmpty) {
    return;
  }
  final uri = Uri.tryParse(trimmedBase);
  if (uri == null || !uri.hasScheme || uri.host.isEmpty) {
    return;
  }

  final isSecure = uri.scheme.toLowerCase() == 'https';
  final origin = '${uri.scheme}://${uri.authority}';
  final cookieHeader =
      'shelfd_token=${Uri.encodeComponent(trimmedJwt)}; Path=/; HttpOnly; SameSite=Lax${isSecure ? '; Secure' : ''}';

  if (Platform.isAndroid) {
    final applied = await QrScannerService().setWebViewCookie(
      url: origin,
      cookie: cookieHeader,
    );
    if (!applied) {
      throw StateError('Failed to apply HttpOnly media cookie on Android WebView.');
    }
    return;
  }

  await WebViewCookieManager().setCookie(
    WebViewCookie(
      name: 'shelfd_token',
      value: trimmedJwt,
      domain: uri.host,
      path: '/',
    ),
  );
}

Future<void> clearHtmlAuthCookies() async {
  if (Platform.environment.containsKey('FLUTTER_TEST')) {
    return;
  }
  if (Platform.isAndroid) {
    await QrScannerService().clearWebViewCookies();
  }
  if ((Platform.isAndroid || Platform.isIOS) && WebViewPlatform.instance != null) {
    try {
      await WebViewCookieManager().clearCookies();
    } catch (_) {}
  }
}

Widget buildHtmlFrame({
  required String viewKey,
  required String url,
  required double width,
  required double height,
  OnReadalongClick? onReadalongClick,
  OnToggleControls? onToggleControls,
}) {
  if (shouldUseHtmlViewStubForCurrentEnvironment()) {
    return stub.buildHtmlFrame(
      viewKey: viewKey,
      url: url,
      width: width,
      height: height,
      onReadalongClick: onReadalongClick,
      onToggleControls: onToggleControls,
    );
  }

  return SizedBox(
    key: Key('html-frame-$viewKey'),
    width: width,
    height: height,
    child: _NativeEpubWebViewFrame(
      key: ValueKey<String>(viewKey),
      viewKey: viewKey,
      url: url,
      onReadalongClick: onReadalongClick,
      onToggleControls: onToggleControls,
    ),
  );
}

void highlightHtmlFrame({
  String? targetViewKey,
  String? bookId,
  String? chapterHref,
  required String fragmentId,
  String activeClass = '-epub-media-overlay-active',
  String playbackActiveClass = '-epub-media-overlay-playing',
  bool playing = true,
}) {
  final hl = ReadalongFrameHighlight(
    targetViewKey: targetViewKey,
    bookId: bookId,
    chapterHref: chapterHref,
    fragmentId: fragmentId,
    activeClass: activeClass,
    playbackActiveClass: playbackActiveClass,
    playing: playing,
  );
  _lastHighlight = hl;

  if (shouldUseHtmlViewStubForCurrentEnvironment()) {
    stub.highlightHtmlFrame(
      targetViewKey: targetViewKey,
      bookId: bookId,
      chapterHref: chapterHref,
      fragmentId: fragmentId,
      activeClass: activeClass,
      playbackActiveClass: playbackActiveClass,
      playing: playing,
    );
    return;
  }

  for (final entry in _liveControllers.values) {
    entry.applyHighlight(hl);
  }
}

class _ManagedWebViewEntry {
  final String viewKey;
  final WebViewController controller;
  String loadedUrl;
  OnReadalongClick? onReadalongClick;
  OnToggleControls? onToggleControls;

  _ManagedWebViewEntry({
    required this.viewKey,
    required this.controller,
    required this.loadedUrl,
    this.onReadalongClick,
    this.onToggleControls,
  });

  void applyHighlight(ReadalongFrameHighlight hl) {
    final isTarget = hl.fragmentId.isNotEmpty &&
        (hl.targetViewKey == null || hl.targetViewKey == viewKey);
    final effectiveFragment = isTarget ? hl.fragmentId : '';
    final effectivePlaying = isTarget ? hl.playing : false;

    final script =
        'if (window.__shelfdHighlightReadalong) { '
        'window.__shelfdHighlightReadalong('
        '${jsonEncode(effectiveFragment)}, '
        '${jsonEncode(hl.activeClass)}, '
        '${jsonEncode(hl.playbackActiveClass)}, '
        '${effectivePlaying ? 'true' : 'false'}'
        '); }';
    controller.runJavaScript(script).catchError((_) {});
  }
}

class _NativeEpubWebViewFrame extends StatefulWidget {
  final String viewKey;
  final String url;
  final OnReadalongClick? onReadalongClick;
  final OnToggleControls? onToggleControls;

  const _NativeEpubWebViewFrame({
    super.key,
    required this.viewKey,
    required this.url,
    this.onReadalongClick,
    this.onToggleControls,
  });

  @override
  State<_NativeEpubWebViewFrame> createState() => _NativeEpubWebViewFrameState();
}

class _NativeEpubWebViewFrameState extends State<_NativeEpubWebViewFrame> {
  late _ManagedWebViewEntry _entry;

  @override
  void initState() {
    super.initState();
    _entry = _acquireControllerEntry();
  }

  @override
  void didUpdateWidget(covariant _NativeEpubWebViewFrame oldWidget) {
    super.didUpdateWidget(oldWidget);
    _entry.onReadalongClick = widget.onReadalongClick;
    _entry.onToggleControls = widget.onToggleControls;
    if (_entry.loadedUrl != widget.url && widget.url.isNotEmpty) {
      _entry.loadedUrl = widget.url;
      final parsed = Uri.tryParse(widget.url);
      if (parsed != null && parsed.hasScheme) {
        _entry.controller.loadRequest(parsed);
      }
    }
  }

  @override
  void dispose() {
    _liveControllers.remove(widget.viewKey);
    super.dispose();
  }

  _ManagedWebViewEntry _acquireControllerEntry() {
    final existing = _liveControllers.remove(widget.viewKey);
    if (existing != null) {
      existing.onReadalongClick = widget.onReadalongClick;
      existing.onToggleControls = widget.onToggleControls;
      _liveControllers[widget.viewKey] = existing;
      if (existing.loadedUrl != widget.url) {
        existing.loadedUrl = widget.url;
        final parsed = Uri.tryParse(widget.url);
        if (parsed != null && parsed.hasScheme) {
          existing.controller.loadRequest(parsed);
        }
      }
      return existing;
    }

    while (_liveControllers.length >= _maxLiveWebViewControllers) {
      _liveControllers.remove(_liveControllers.keys.first);
    }

    final controller = WebViewController();
    controller.setJavaScriptMode(JavaScriptMode.unrestricted);
    controller.setBackgroundColor(Colors.white);

    final platformController = controller.platform;
    if (platformController is AndroidWebViewController) {
      platformController.setUseWideViewPort(false);
      platformController.enableZoom(false);
      platformController.setTextZoom(100);
      platformController.setVerticalScrollBarEnabled(false);
      platformController.setHorizontalScrollBarEnabled(false);
      platformController.setAllowFileAccess(false);
      platformController.setAllowContentAccess(false);
    }

    late final _ManagedWebViewEntry created;
    controller.addJavaScriptChannel(
      'ShelfdBridge',
      onMessageReceived: (JavaScriptMessage msg) {
        try {
          final decoded = jsonDecode(msg.message);
          if (decoded is! Map<String, dynamic>) return;
          final type = decoded['type']?.toString() ?? '';
          if (type == 'shelfd:readalong-click') {
            final fragmentId = decoded['fragmentId']?.toString() ?? '';
            final cb = created.onReadalongClick;
            if (cb != null && fragmentId.isNotEmpty) {
              cb(fragmentId);
            } else {
              created.onToggleControls?.call();
            }
          } else if (type == 'shelfd:toggle-controls') {
            created.onToggleControls?.call();
          }
        } catch (_) {}
      },
    );

    controller.setNavigationDelegate(
      NavigationDelegate(
        onPageFinished: (_) {
          final hl = _lastHighlight;
          if (hl != null) {
            created.applyHighlight(hl);
          }
        },
        onNavigationRequest: (NavigationRequest request) {
          if (request.url == created.loadedUrl || request.url == 'about:blank') {
            return NavigationDecision.navigate;
          }
          return NavigationDecision.prevent;
        },
      ),
    );

    created = _ManagedWebViewEntry(
      viewKey: widget.viewKey,
      controller: controller,
      loadedUrl: widget.url,
      onReadalongClick: widget.onReadalongClick,
      onToggleControls: widget.onToggleControls,
    );
    _liveControllers[widget.viewKey] = created;

    final parsed = Uri.tryParse(widget.url);
    if (parsed != null && parsed.hasScheme) {
      controller.loadRequest(parsed);
    }

    return created;
  }

  @override
  Widget build(BuildContext context) {
    return WebViewWidget(controller: _entry.controller);
  }
}
