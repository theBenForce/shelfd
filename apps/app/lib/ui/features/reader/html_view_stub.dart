import 'package:flutter/material.dart';

Widget buildHtmlFrame({
  required String viewKey,
  required String url,
  required double width,
  required double height,
}) {
  return Container(
    width: width,
    height: height,
    color: Colors.black,
    child: const Center(
      child: Text(
        'Fixed-layout view is optimized for web or embedded browser.',
        style: TextStyle(color: Colors.white70),
      ),
    ),
  );
}
