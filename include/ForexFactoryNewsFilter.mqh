//+------------------------------------------------------------------+
//|                                     ForexFactoryNewsFilter.mqh   |
//|                        Copyright 2026, forexfactory-go OpenSource |
//|                     https://github.com/Nosvemos/forexfactory-go  |
//+------------------------------------------------------------------+
#property copyright "ForexFactory-Go"
#property link      "https://github.com/Nosvemos/forexfactory-go"
#property strict

//+------------------------------------------------------------------+
//| Struct representing an incoming economic event parsed from CSV   |
//+------------------------------------------------------------------+
struct FFNewsEvent
{
   string currency;
   int    minutesLeft;
   string impact;
   datetime releaseTime;
   string title;
   string forecast;
   string previous;
};

//+------------------------------------------------------------------+
//| ForexFactoryNewsFilter Class                                     |
//+------------------------------------------------------------------+
class CForexFactoryNewsFilter
{
private:
   string m_fileName;
   datetime m_lastReadTime;

public:
   CForexFactoryNewsFilter(string fileName = "ff_news_filter.csv")
   {
      m_fileName = fileName;
      m_lastReadTime = 0;
   }

   // Checks if a high-impact news event is upcoming within 'minutesBefore' or passed within 'minutesAfter'
   bool IsNewsRestricted(string symbolCurrency, int minutesBefore = 30, int minutesAfter = 15, string minImpact = "High")
   {
      int fileHandle = FileOpen(m_fileName, FILE_READ|FILE_CSV|FILE_ANSI, ',');
      if(fileHandle == INVALID_HANDLE)
      {
         Print("[FF-NewsFilter] Notice: Unable to open ", m_fileName, " in Terminal MQL Files directory.");
         return false;
      }

      // Skip header line
      FileReadString(fileHandle); // Currency
      FileReadString(fileHandle); // MinutesLeft
      FileReadString(fileHandle); // Impact
      FileReadString(fileHandle); // Timestamp
      FileReadString(fileHandle); // Title
      FileReadString(fileHandle); // Forecast
      FileReadString(fileHandle); // Previous

      while(!FileIsEnding(fileHandle))
      {
         string cur       = FileReadString(fileHandle);
         string minsStr   = FileReadString(fileHandle);
         string imp       = FileReadString(fileHandle);
         string tsStr     = FileReadString(fileHandle);
         string title     = FileReadString(fileHandle);
         string forecast  = FileReadString(fileHandle);
         string previous  = FileReadString(fileHandle);

         if(cur == "") continue;

         int minsLeft = (int)StringToInteger(minsStr);

         // Check if currency matches either base or quote currency of symbol
         if(StringFind(symbolCurrency, cur) >= 0 || cur == "ALL")
         {
            // Check impact level
            if(IsImpactEligible(imp, minImpact))
            {
               // Check if time is within restriction window
               if(minsLeft <= minutesBefore && minsLeft >= -minutesAfter)
               {
                  FileClose(fileHandle);
                  return true; // Trading restricted!
               }
            }
         }
      }

      FileClose(fileHandle);
      return false; // Safe to trade
   }

private:
   bool IsImpactEligible(string actualImpact, string targetMinImpact)
   {
      int actualWeight = GetImpactWeight(actualImpact);
      int targetWeight = GetImpactWeight(targetMinImpact);
      return actualWeight >= targetWeight;
   }

   int GetImpactWeight(string impact)
   {
      if(impact == "High") return 3;
      if(impact == "Medium") return 2;
      if(impact == "Low") return 1;
      return 0;
   }
};
