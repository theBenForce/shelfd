class ServerConnectInfo {
  final String serverName;
  final String version;
  final String baseUrl;
  final String? apiBase;
  final String? mcpBase;
  final List<String> capabilities;
  final String? defaultUsername;

  const ServerConnectInfo({
    required this.serverName,
    required this.version,
    required this.baseUrl,
    this.apiBase,
    this.mcpBase,
    required this.capabilities,
    this.defaultUsername,
  });

  factory ServerConnectInfo.fromJson(Map<String, dynamic> json) {
    return ServerConnectInfo(
      serverName: json['server_name'] as String? ?? 'shelfd',
      version: json['version'] as String? ?? '0.1.0',
      baseUrl: json['base_url'] as String? ?? '',
      apiBase: json['api_base'] as String?,
      mcpBase: json['mcp_base'] as String?,
      capabilities: (json['capabilities'] as List<dynamic>?)
              ?.map((e) => e.toString())
              .toList() ??
          const [],
      defaultUsername: json['default_username'] as String?,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'server_name': serverName,
      'version': version,
      'base_url': baseUrl,
      if (apiBase != null) 'api_base': apiBase,
      if (mcpBase != null) 'mcp_base': mcpBase,
      'capabilities': capabilities,
      if (defaultUsername != null) 'default_username': defaultUsername,
    };
  }
}
