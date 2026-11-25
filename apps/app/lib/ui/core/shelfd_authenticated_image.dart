import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../data/services/api_service.dart';
import '../../data/utils/url_normalizer.dart';
import '../state/providers.dart';

class ShelfdAuthenticatedImage extends StatelessWidget {
  final String imageUrl;
  final Map<String, String>? extraHeaders;
  final BoxFit? fit;
  final double? width;
  final double? height;
  final AlignmentGeometry alignment;
  final FilterQuality filterQuality;
  final ImageErrorWidgetBuilder? errorBuilder;
  final ImageLoadingBuilder? loadingBuilder;

  const ShelfdAuthenticatedImage({
    super.key,
    required this.imageUrl,
    this.extraHeaders,
    this.fit,
    this.width,
    this.height,
    this.alignment = Alignment.center,
    this.filterQuality = FilterQuality.low,
    this.errorBuilder,
    this.loadingBuilder,
  });

  @override
  Widget build(BuildContext context) {
    ApiService? apiService;
    try {
      final container = ProviderScope.containerOf(context, listen: true);
      apiService = container.read(apiServiceProvider);
    } catch (_) {
      apiService = null;
    }

    final baseUrl = apiService?.baseUrl ?? '';
    final resolvedUrl = _resolveUrl(imageUrl, baseUrl);
    final headers = buildOriginScopedHeaders(
      targetUrl: resolvedUrl,
      serverBaseUrl: baseUrl,
      authToken: apiService?.token,
      extraHeaders: extraHeaders,
    );

    return Image.network(
      resolvedUrl,
      headers: headers,
      fit: fit,
      width: width,
      height: height,
      alignment: alignment,
      filterQuality: filterQuality,
      errorBuilder: errorBuilder,
      loadingBuilder: loadingBuilder,
    );
  }

  static String _resolveUrl(String rawUrl, String baseUrl) {
    final trimmed = rawUrl.trim();
    if (trimmed.startsWith('/') && !trimmed.startsWith('//')) {
      final cleanBase = baseUrl.trim();
      if (cleanBase.isNotEmpty) {
        final noSlash = cleanBase.endsWith('/')
            ? cleanBase.substring(0, cleanBase.length - 1)
            : cleanBase;
        return '$noSlash$trimmed';
      }
    }
    return trimmed;
  }

  static Map<String, String>? buildOriginScopedHeaders({
    required String targetUrl,
    required String serverBaseUrl,
    required String? authToken,
    Map<String, String>? extraHeaders,
    bool isWeb = kIsWeb,
  }) {
    final merged = <String, String>{
      if (extraHeaders != null) ...extraHeaders,
    };

    final cleanToken = authToken?.trim();
    if (!isWeb &&
        cleanToken != null &&
        cleanToken.isNotEmpty &&
        UrlNormalizer.isSameOrigin(serverBaseUrl, targetUrl)) {
      merged.putIfAbsent('Authorization', () => 'Bearer $cleanToken');
    }

    return merged.isEmpty ? null : merged;
  }
}
