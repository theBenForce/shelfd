import 'package:flutter/material.dart';
import 'tokens.dart';

class Responsive {
  Responsive._();

  static bool isMobile(BuildContext context) =>
      MediaQuery.sizeOf(context).width <= AppTokens.mobileBreakpoint;

  static bool isTablet(BuildContext context) {
    final width = MediaQuery.sizeOf(context).width;
    return width > AppTokens.mobileBreakpoint && width <= AppTokens.tabletBreakpoint;
  }

  static bool isDesktop(BuildContext context) =>
      MediaQuery.sizeOf(context).width > AppTokens.tabletBreakpoint;

  static double horizontalPadding(BuildContext context) {
    if (isDesktop(context)) return AppTokens.space48;
    if (isTablet(context)) return AppTokens.space32;
    return AppTokens.space20;
  }
}
