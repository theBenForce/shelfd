class ServerConnectInfo {
  final String serverName;
  final String version;
  final String baseUrl;
  final List<String> capabilities;

  const ServerConnectInfo({
    required this.serverName,
    required this.version,
    required this.baseUrl,
    required this.capabilities,
  });

  factory ServerConnectInfo.fromJson(Map<String, dynamic> json) {
    return ServerConnectInfo(
      serverName: json['server_name'] as String? ?? 'shelfd',
      version: json['version'] as String? ?? '0.1.0',
      baseUrl: json['base_url'] as String? ?? '',
      capabilities: (json['capabilities'] as List<dynamic>?)
              ?.map((e) => e.toString())
              .toList() ??
          const [],
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'server_name': serverName,
      'version': version,
      'base_url': baseUrl,
      'capabilities': capabilities,
    };
  }
}
