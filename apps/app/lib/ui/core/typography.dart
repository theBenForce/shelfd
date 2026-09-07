import 'package:flutter/material.dart';
import 'package:google_fonts/google_fonts.dart';
import 'tokens.dart';

class AppTypography {
  AppTypography._();

  static TextStyle titleSerif({
    double fontSize = 22.0,
    FontWeight fontWeight = FontWeight.w600,
    Color color = AppTokens.charcoalInk,
  }) {
    return GoogleFonts.sourceSerif4(
      fontSize: fontSize,
      fontWeight: fontWeight,
      color: color,
      letterSpacing: -0.2,
    );
  }

  static TextStyle bodySans({
    double fontSize = 15.0,
    FontWeight fontWeight = FontWeight.w400,
    Color color = AppTokens.mutedCopy,
    double lineHeight = 1.5,
  }) {
    return GoogleFonts.inter(
      fontSize: fontSize,
      fontWeight: fontWeight,
      color: color,
      height: lineHeight,
    );
  }

  static TextStyle labelCaps({
    double fontSize = 11.0,
    FontWeight fontWeight = FontWeight.w600,
    Color color = AppTokens.mutedCopy,
  }) {
    return GoogleFonts.inter(
      fontSize: fontSize,
      fontWeight: fontWeight,
      color: color,
      letterSpacing: 0.8,
    );
  }

  static TextStyle readerText({
    required double fontSize,
    required double lineHeight,
    required bool isSerif,
    required Color color,
  }) {
    if (isSerif) {
      return GoogleFonts.sourceSerif4(
        fontSize: fontSize,
        height: lineHeight,
        color: color,
      );
    }
    return GoogleFonts.inter(
      fontSize: fontSize,
      height: lineHeight,
      color: color,
    );
  }
}
